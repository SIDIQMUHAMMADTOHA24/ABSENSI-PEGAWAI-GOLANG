package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

type debugResetReq struct {
	UserID string `json:"user_id,omitempty"` // opsional: kalau kosong, ambil dari auth
	TZ     string `json:"tz,omitempty"`      // opsional: default Asia/Jakarta
}

type debugResetResp struct {
	OK      bool   `json:"ok"`
	UserID  string `json:"user_id"`
	Date    string `json:"date"` // yyyy-mm-dd (zona lokal)
	Message string `json:"message,omitempty"`
}

// DebugResetToday: DEV ONLY – reset status absensi HARI INI (check_in/out & worked_seconds).
func (h *AttendanceHandler) DebugResetToday(w http.ResponseWriter, r *http.Request) {
	// Lindungi: jangan boleh di production
	if os.Getenv("APP_ENV") == "production" {
		http.Error(w, "forbidden in production", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req debugResetReq
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Cari user_id: pakai body, kalau kosong ambil dari auth yang biasa kamu pakai
	userID := req.UserID
	if userID == "" {
		var ok bool
		userID, ok = userIDFromRequest(r)
		if !ok || userID == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
	}

	// Zona waktu: default Asia/Jakarta
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if req.TZ != "" {
		if l, err := time.LoadLocation(req.TZ); err == nil {
			loc = l
		}
	}
	today := time.Now().In(loc).Format("2006-01-02")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	affected, err := h.Attendance.ResetToday(ctx, userID, today)
	if err != nil {
		http.Error(w, "reset failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := debugResetResp{
		OK:     true,
		UserID: userID,
		Date:   today,
	}
	if affected == 0 {
		resp.Message = "no row updated (maybe already empty?)"
	} else {
		resp.Message = "reset done"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ===== Helper ambil user id dari request =====
// Sesuaikan dgn cara kamu ambil user di Status/CheckIn/CheckOut.
// Di sini contoh minimal: ambil dari context "user_id".
func userIDFromRequest(r *http.Request) (string, bool) {
	v := r.Context().Value("user_id")
	if s, ok := v.(string); ok && s != "" {
		return s, true
	}
	return "", false
}
