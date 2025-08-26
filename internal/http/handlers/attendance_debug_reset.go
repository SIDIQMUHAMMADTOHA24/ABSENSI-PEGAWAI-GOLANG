package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
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
	// 1) Context (kalau nanti kamu punya middleware yang inject user_id)
	if v := r.Context().Value("user_id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s, true
		}
	}

	// 2) Authorization header (Bearer JWT)
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	tokenStr := strings.TrimSpace(parts[1])

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Server misconfigured — lebih aman kembalikan unauthorized.
		return "", false
	}

	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		// (opsional) validasi method HS256, dsb.
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return "", false
	}

	// Cari claim user id di salah satu key ini. Tokenmu di contoh punya "sub".
	if claims, ok := tok.Claims.(jwt.MapClaims); ok {
		for _, key := range []string{"uid", "user_id", "sub", "id"} {
			if v, ok := claims[key]; ok {
				switch vv := v.(type) {
				case string:
					if vv != "" {
						return vv, true
					}
				case float64:
					return strconv.FormatInt(int64(vv), 10), true
				}
			}
		}
	}
	return "", false
}
