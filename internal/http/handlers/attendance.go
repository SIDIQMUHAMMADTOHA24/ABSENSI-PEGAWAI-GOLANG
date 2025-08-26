package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"absensi/internal/repo"
	"absensi/internal/util"
	"absensi/internal/util/imgutil"
)

type AttendanceHandler struct {
	Users      *repo.UserRepo
	Attendance *repo.AttendanceRepo
}

type officeCfgResp struct {
	OfficeLat float64 `json:"office_lat"`
	OfficeLng float64 `json:"office_lng"`
	RadiusM   float64 `json:"radius_m"`
}

func (h *AttendanceHandler) GetOfficeConfig(w http.ResponseWriter, r *http.Request) {

	if _, _, ok := mustAuth(w, r); !ok {
		return
	}

	resp := officeCfgResp{
		OfficeLat: util.OfficeLat,
		OfficeLng: util.OfficeLng,
		RadiusM:   util.OfficeRadiusM,
	}
	writeJSON(w, http.StatusOK, resp)
}

type posReq struct {
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	SelfieBase64 string  `json:"selfie_base64"` // wajib
}

func (h *AttendanceHandler) Status(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req posReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	officeDate := util.OfficeDate(now).Format("2006-01-02")

	dist := util.HaversineMeters(req.Lat, req.Lng, util.OfficeLat, util.OfficeLng)
	inside := util.InsideRadius(dist)

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	day, err := h.Attendance.GetByUserAndDate(ctx, uid, util.OfficeDate(now))
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	var checkInAt, checkOutAt *time.Time
	if day.CheckInAt.Valid {
		t := day.CheckInAt.Time
		checkInAt = &t
	}
	if day.CheckOutAt.Valid {
		t := day.CheckOutAt.Time
		checkOutAt = &t
	}

	next := ""
	switch {
	case !day.CheckInAt.Valid:
		next = "check_in"
	case day.CheckInAt.Valid && !day.CheckOutAt.Valid:
		next = "check_out"
	default:
		next = ""
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"inside_radius": inside,
		"distance_m":    round1(dist),
		"today": map[string]any{
			"date":           officeDate,
			"check_in_at":    toRFC3339(checkInAt),
			"check_out_at":   toRFC3339(checkOutAt),
			"worked_seconds": day.WorkedSeconds,
		},
		"next_action": next,
	})
}

func (h *AttendanceHandler) CheckIn(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req posReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.SelfieBase64) == "" {
		http.Error(w, "selfie required", http.StatusBadRequest)
		return
	}

	// normalisasi + resize → JPEG base64
	normB64, err := imgutil.NormalizeBase64(req.SelfieBase64)
	if err != nil {
		http.Error(w, "invalid selfie: "+err.Error(), http.StatusBadRequest)
		return
	}

	dist := util.HaversineMeters(req.Lat, req.Lng, util.OfficeLat, util.OfficeLng)
	if !util.InsideRadius(dist) {
		writeJSON(w, 422, map[string]any{
			"error": map[string]any{
				"code":    "outside_radius",
				"details": map[string]any{"distance_m": round1(dist), "radius_m": util.OfficeRadiusM},
			},
		})
		return
	}

	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// NOTE: repo perlu diubah terima argumen foto (lihat catatan di bawah)
	ad, err := h.Attendance.DoCheckIn(ctx, uid, util.OfficeDate(now), now, req.Lat, req.Lng, dist, normB64)
	if err != nil {
		// kemungkinan sudah check-in
		writeJSON(w, 409, map[string]any{"error": map[string]any{"code": "already_checked_in"}})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"result":     "checked_in",
		"distance_m": round1(dist),
		"today": map[string]any{
			"date":           util.OfficeDate(now).Format("2006-01-02"),
			"check_in_at":    toRFC3339(optTime(ad.CheckInAt)),
			"check_out_at":   toRFC3339(optTime(ad.CheckOutAt)),
			"worked_seconds": ad.WorkedSeconds,
		},
		"next_action": "check_out",
	})
}

func (h *AttendanceHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid, _, ok := mustAuth(w, r)
	if !ok {
		return
	}

	var req posReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.SelfieBase64) == "" {
		http.Error(w, "selfie required", http.StatusBadRequest)
		return
	}

	normB64, err := imgutil.NormalizeBase64(req.SelfieBase64)
	if err != nil {
		http.Error(w, "invalid selfie: "+err.Error(), http.StatusBadRequest)
		return
	}

	dist := util.HaversineMeters(req.Lat, req.Lng, util.OfficeLat, util.OfficeLng)
	if !util.InsideRadius(dist) {
		writeJSON(w, 422, map[string]any{
			"error": map[string]any{
				"code":    "outside_radius",
				"details": map[string]any{"distance_m": round1(dist), "radius_m": util.OfficeRadiusM},
			},
		})
		return
	}

	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// NOTE: repo perlu diubah terima argumen foto (lihat catatan di bawah)
	ad, err := h.Attendance.DoCheckOut(ctx, uid, util.OfficeDate(now), now, req.Lat, req.Lng, dist, normB64)
	if err != nil {
		// belum check-in atau sudah check-out
		writeJSON(w, 409, map[string]any{"error": map[string]any{"code": "not_checked_in_yet_or_already_checked_out"}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     "checked_out",
		"distance_m": round1(dist),
		"today": map[string]any{
			"date":           util.OfficeDate(now).Format("2006-01-02"),
			"check_in_at":    toRFC3339(optTime(ad.CheckInAt)),
			"check_out_at":   toRFC3339(optTime(ad.CheckOutAt)),
			"worked_seconds": ad.WorkedSeconds,
		},
		"next_action": nil,
	})
}

// ===== helpers (auth & json) =====

func mustAuth(w http.ResponseWriter, r *http.Request) (userID, username string, ok bool) {
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return "", "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	uid, usr, err := util.ParseAccessToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return "", "", false
	}
	return uid, usr, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func toRFC3339(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func optTime(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
