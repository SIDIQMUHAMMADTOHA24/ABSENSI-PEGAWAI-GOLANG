package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"absensi/internal/repo"
)

type LeaveHandler struct {
	Leaves *repo.LeaveRepo
}

// ===== GET /leave/quota  (tahun berjalan) =====

type quotaResp struct {
	Year          int `json:"year"`
	QuotaDays     int `json:"quota_days"` // default 12
	UsedDays      int `json:"used_days"`  // hanya approved
	RemainingDays int `json:"remaining_days"`
}

func (h *LeaveHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	// Tahun berjalan (pakai zona lokal kantor supaya konsisten)
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	year := now.Year()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const quota = 12

	used, err := h.Leaves.SumApprovedDays(ctx, userID, year)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if used < 0 {
		used = 0
	}

	resp := quotaResp{
		Year:          year,
		QuotaDays:     quota,
		UsedDays:      used,
		RemainingDays: quota - used,
	}
	writeJSON(w, http.StatusOK, resp)
}

// ===== POST /leave/cuti/request  =====

type cutiReq struct {
	Reason    string `json:"reason"`     // keperluan
	StartDate string `json:"start_date"` // "yyyy-mm-dd"
	EndDate   string `json:"end_date"`   // "yyyy-mm-dd"
}
type cutiResp struct {
	RequestID string    `json:"request_id"`
	Status    string    `json:"status"` // always "pending"
	Days      int       `json:"days"`
	StartDate string    `json:"start_date"`
	EndDate   string    `json:"end_date"`
	Quota     quotaResp `json:"quota_snapshot"`
}

func (h *LeaveHandler) RequestCuti(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req cutiReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	start, err1 := time.ParseInLocation("2006-01-02", req.StartDate, loc)
	end, err2 := time.ParseInLocation("2006-01-02", req.EndDate, loc)
	if err1 != nil || err2 != nil || end.Before(start) {
		http.Error(w, "invalid date range", http.StatusBadRequest)
		return
	}

	// hitung hari inklusif
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}

	// Kuota tahun berjalan
	year := start.Year() // diasumsikan cuti dalam satu tahun yang sama
	const quota = 12

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	used, err := h.Leaves.SumApprovedDays(ctx, userID, year)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	remain := quota - used
	if days > remain {
		// Tidak potong kuota sekarang, tapi kita batasi
		// agar pengajuan tidak melebihi sisa kuota.
		writeJSON(w, 422, map[string]any{
			"error": map[string]any{
				"code": "quota_exceeded",
				"details": map[string]any{
					"requested_days": days,
					"remaining_days": remain,
				},
			},
		})
		return
	}

	// (opsional) Tolak kalau overlap dengan pending/approved lainnya
	// overlap, err := h.Leaves.HasOverlap(ctx, userID, start, end)
	// if err != nil { http.Error(w, "db error", 500); return }
	// if overlap {
	//   writeJSON(w, 409, map[string]any{"error": map[string]any{"code":"date_overlap"}})
	//   return
	// }

	id, err := h.Leaves.CreateCutiPending(ctx, userID, start, end, days, req.Reason)
	if err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	resp := cutiResp{
		RequestID: id,
		Status:    "pending",
		Days:      days,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		Quota: quotaResp{
			Year:          year,
			QuotaDays:     quota,
			UsedDays:      used,
			RemainingDays: remain, // belum dipotong—dipotong saat APPROVE
		},
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ===== POST /leave/cuti/approve & /leave/cuti/reject =====

type cutiDecisionReq struct {
	RequestID string `json:"request_id"`
}

type cutiDecisionResp struct {
	RequestID     string    `json:"request_id"`
	Status        string    `json:"status"` // approved / rejected
	Reason        string    `json:"reason,omitempty"`
	Days          int       `json:"days"`
	StartDate     string    `json:"start_date"`
	EndDate       string    `json:"end_date"`
	QuotaSnapshot quotaResp `json:"quota_snapshot"`
}

// POST /leave/cuti/approve
func (h *LeaveHandler) ApproveCuti(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req cutiDecisionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RequestID == "" {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	lr, err := h.Leaves.GetLeaveByID(ctx, req.RequestID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if lr.ID == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Sesuai requirement: approve via token usernya sendiri.
	if lr.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if lr.Kind != "cuti" {
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	if lr.Status != "pending" {
		http.Error(w, "conflict: not pending", http.StatusConflict)
		return
	}

	year := lr.StartDate.Year()
	const quota = 12

	used, err := h.Leaves.SumApprovedDays(ctx, lr.UserID, year)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	remain := quota - used
	if lr.Days > remain {
		writeJSON(w, 422, map[string]any{
			"error": map[string]any{
				"code": "quota_exceeded",
				"details": map[string]any{
					"requested_days": lr.Days,
					"remaining_days": remain,
				},
			},
		})
		return
	}

	okUpdated, err := h.Leaves.ApproveCutiPending(ctx, lr.ID, time.Now().UTC())
	if err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	if !okUpdated {
		http.Error(w, "conflict: already decided", http.StatusConflict)
		return
	}

	newUsed := used + lr.Days
	resp := cutiDecisionResp{
		RequestID: lr.ID,
		Status:    "approved",
		Reason:    lr.Reason.String,
		Days:      lr.Days,
		StartDate: lr.StartDate.Format("2006-01-02"),
		EndDate:   lr.EndDate.Format("2006-01-02"),
		QuotaSnapshot: quotaResp{
			Year:          year,
			QuotaDays:     quota,
			UsedDays:      newUsed,
			RemainingDays: quota - newUsed,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /leave/cuti/reject
func (h *LeaveHandler) RejectCuti(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req cutiDecisionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RequestID == "" {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	lr, err := h.Leaves.GetLeaveByID(ctx, req.RequestID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if lr.ID == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if lr.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if lr.Kind != "cuti" {
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	if lr.Status != "pending" {
		http.Error(w, "conflict: not pending", http.StatusConflict)
		return
	}

	okUpdated, err := h.Leaves.RejectCutiPending(ctx, lr.ID, time.Now().UTC())
	if err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	if !okUpdated {
		http.Error(w, "conflict: already decided", http.StatusConflict)
		return
	}

	year := lr.StartDate.Year()
	const quota = 12
	used, err := h.Leaves.SumApprovedDays(ctx, lr.UserID, year)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	resp := cutiDecisionResp{
		RequestID: lr.ID,
		Status:    "rejected",
		Reason:    lr.Reason.String,
		Days:      lr.Days,
		StartDate: lr.StartDate.Format("2006-01-02"),
		EndDate:   lr.EndDate.Format("2006-01-02"),
		QuotaSnapshot: quotaResp{
			Year:          year,
			QuotaDays:     quota,
			UsedDays:      used,         // tidak berubah
			RemainingDays: quota - used, // tidak berubah
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

type cutiListItem struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"` // pending|approved|rejected
	Reason    string  `json:"reason,omitempty"`
	StartDate string  `json:"start_date"` // yyyy-mm-dd
	EndDate   string  `json:"end_date"`   // yyyy-mm-dd
	Days      int     `json:"days"`
	CreatedAt string  `json:"created_at"`           // RFC3339 UTC
	DecidedAt *string `json:"decided_at,omitempty"` // RFC3339 UTC (nullable)
}

type cutiListResp struct {
	Year         int            `json:"year"`
	StatusFilter string         `json:"status_filter"` // all|pending|approved|rejected
	Items        []cutiListItem `json:"items"`
}

func (h *LeaveHandler) ListCuti(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	switch status {
	case "all", "pending", "approved", "rejected":
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	// tahun default: tahun berjalan (zona kantor)
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	year := now.Year()

	if y := q.Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		} else {
			http.Error(w, "invalid year", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.Leaves.ListCutiByYearStatus(ctx, userID, year, status)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	toPtr := func(nt sql.NullTime) *string {
		if nt.Valid {
			s := nt.Time.UTC().Format(time.RFC3339)
			return &s
		}
		return nil
	}

	items := make([]cutiListItem, 0, len(rows))
	for _, lr := range rows {
		items = append(items, cutiListItem{
			ID:        lr.ID,
			Status:    lr.Status,
			Reason:    lr.Reason.String,
			StartDate: lr.StartDate.Format("2006-01-02"),
			EndDate:   lr.EndDate.Format("2006-01-02"),
			Days:      lr.Days,
			CreatedAt: lr.CreatedAt.UTC().Format(time.RFC3339),
			DecidedAt: toPtr(lr.DecidedAt),
		})
	}

	resp := cutiListResp{
		Year:         year,
		StatusFilter: status,
		Items:        items,
	}
	writeJSON(w, http.StatusOK, resp)
}

type sickReq struct {
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	DoctorNoteBase64 string `json:"doctor_note_base64"`
	Reason           string `json:"reason,omitempty"`
}

type sickResp struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"` // "pending"
	Days      int    `json:"days"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (h *LeaveHandler) RequestSakit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req sickReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.DoctorNoteBase64 == "" {
		http.Error(w, "doctor_note_base64 required", http.StatusBadRequest)
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	start, err1 := time.ParseInLocation("2006-01-02", req.StartDate, loc)
	end, err2 := time.ParseInLocation("2006-01-02", req.EndDate, loc)
	if err1 != nil || err2 != nil || end.Before(start) {
		http.Error(w, "invalid date range", http.StatusBadRequest)
		return
	}

	// hitung hari inklusif
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		http.Error(w, "invalid days", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	id, err := h.Leaves.CreateSakitPending(ctx, userID, start, end, days, req.Reason, req.DoctorNoteBase64)
	if err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, sickResp{
		RequestID: id,
		Status:    "pending",
		Days:      days,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	})
}

// ===== POST /leave/sakit/{id}/approve =====
func (h *LeaveHandler) ApproveSakit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// bearer user sendiri (by design saat ini)
	if _, _, ok := mustAuth(w, r); !ok {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	aff, err := h.Leaves.ApproveSick(ctx, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if aff == 0 {
		http.Error(w, "not found or already decided", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result": "approved",
		"id":     id,
	})
}

// ===== POST /leave/sakit/{id}/reject =====
func (h *LeaveHandler) RejectSakit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, _, ok := mustAuth(w, r); !ok {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	aff, err := h.Leaves.RejectSick(ctx, id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if aff == 0 {
		http.Error(w, "not found or already decided", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result": "rejected",
		"id":     id,
	})
}

// ===== GET /leave/sakit/list?status=all|pending|approved|rejected&year=YYYY =====

type sickListItem struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Reason    string  `json:"reason,omitempty"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Days      int     `json:"days"`
	CreatedAt string  `json:"created_at"`           // RFC3339 UTC
	DecidedAt *string `json:"decided_at,omitempty"` // RFC3339 UTC
	HasProof  bool    `json:"has_proof"`
}

type sickListResp struct {
	Year         int            `json:"year"`
	StatusFilter string         `json:"status_filter"`
	Items        []sickListItem `json:"items"`
}

func (h *LeaveHandler) ListSakit(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	switch status {
	case "all", "pending", "approved", "rejected":
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	year := now.Year()

	if y := q.Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		} else {
			http.Error(w, "invalid year", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.Leaves.ListSakitByYearStatus(ctx, userID, year, status)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	toPtr := func(nt sql.NullTime) *string {
		if nt.Valid {
			s := nt.Time.UTC().Format(time.RFC3339)
			return &s
		}
		return nil
	}

	items := make([]sickListItem, 0, len(rows))
	for _, sr := range rows {
		items = append(items, sickListItem{
			ID:        sr.ID,
			Status:    sr.Status,
			Reason:    sr.Reason.String,
			StartDate: sr.StartDate.Format("2006-01-02"),
			EndDate:   sr.EndDate.Format("2006-01-02"),
			Days:      sr.Days,
			CreatedAt: sr.CreatedAt.UTC().Format(time.RFC3339),
			DecidedAt: toPtr(sr.DecidedAt),
			HasProof:  sr.ProofBase64.Valid && sr.ProofBase64.String != "",
		})
	}

	writeJSON(w, http.StatusOK, sickListResp{
		Year:         year,
		StatusFilter: status,
		Items:        items,
	})
}
