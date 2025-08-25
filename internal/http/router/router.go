package router

import (
	"database/sql"
	"net/http"

	"absensi/internal/http/handlers"
	"absensi/internal/repo"
)

func New(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	uh := &handlers.AuthHandler{
		Users:       repo.NewUserRepo(db),
		RefreshRepo: repo.NewRefreshRepo(db),
	}

	mux.HandleFunc("POST /register", uh.Register)
	mux.HandleFunc("POST /login", uh.Login)
	mux.HandleFunc("POST /refresh", uh.RefreshToken)
	mux.HandleFunc("POST /logout", uh.Logout)
	mux.HandleFunc("GET /get-user", uh.GetUser)

	ah := &handlers.AttendanceHandler{
		Users:      repo.NewUserRepo(db),
		Attendance: repo.NewAttendanceRepo(db),
	}
	mux.HandleFunc("GET /config/office", ah.GetOfficeConfig)
	mux.HandleFunc("POST /attendance/status", ah.Status)
	mux.HandleFunc("POST /attendance/check-in", ah.CheckIn)
	mux.HandleFunc("POST /attendance/check-out", ah.CheckOut)

	mux.HandleFunc("POST /attendance/debug/reset-today", ah.DebugResetToday)

	return mux
}
