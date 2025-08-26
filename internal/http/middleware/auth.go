package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Ambil user_id dari context ATAU dari Authorization: Bearer <jwt>
func userIDFromRequest(r *http.Request) (string, bool) {
	// 1) Coba dari context (kalau suatu saat kamu pakai middleware)
	if v := r.Context().Value("user_id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s, true
		}
	}

	// 2) Fallback: parse Authorization header (Bearer JWT)
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
		// server misconfigured; daripada panic, balikin unauthorized saja
		return "", false
	}

	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		// kalau pakai HS256, tidak perlu cek method di sini (boleh ditambah)
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return "", false
	}

	// Ambil claim user id. Sesuaikan dengan isi token kamu:
	// coba "uid", "user_id", "sub", "id" secara berurutan.
	if claims, ok := tok.Claims.(jwt.MapClaims); ok {
		for _, key := range []string{"uid", "user_id", "sub", "id"} {
			if v, ok := claims[key]; ok {
				switch vv := v.(type) {
				case string:
					if vv != "" {
						return vv, true
					}
				case float64:
					// kalau id disimpan sebagai angka
					return strconv.FormatInt(int64(vv), 10), true
				}
			}
		}
	}

	return "", false
}
