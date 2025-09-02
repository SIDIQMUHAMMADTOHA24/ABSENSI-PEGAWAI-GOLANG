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
	ah := &handlers.AttendanceHandler{
		Users:      repo.NewUserRepo(db),
		Attendance: repo.NewAttendanceRepo(db),
	}

	lh := &handlers.LeaveHandler{
		Leaves: repo.NewLeaveRepo(db),
	}

	mux.HandleFunc("POST /register", uh.Register)
	mux.HandleFunc("POST /login", uh.Login)
	mux.HandleFunc("POST /refresh", uh.RefreshToken)
	mux.HandleFunc("POST /logout", uh.Logout)
	mux.HandleFunc("GET /get-user", uh.GetUser)

	mux.HandleFunc("GET /config/office", ah.GetOfficeConfig)
	mux.HandleFunc("POST /attendance/status", ah.Status)
	mux.HandleFunc("POST /attendance/check-in", ah.CheckIn)
	mux.HandleFunc("POST /attendance/check-out", ah.CheckOut)
	mux.HandleFunc("POST /attendance/debug/reset-today", ah.DebugResetToday)
	mux.HandleFunc("GET /attendance/marks", ah.GetMarks)
	mux.HandleFunc("GET /attendance/day", ah.GetDay)

	mux.HandleFunc("GET /leave/quota", lh.GetQuota)
	mux.HandleFunc("POST /leave/cuti/request", lh.RequestCuti)
	mux.HandleFunc("GET /leave/cuti/list", lh.ListCuti)
	mux.HandleFunc("POST /leave/cuti/approve", lh.ApproveCuti)
	mux.HandleFunc("POST /leave/cuti/reject", lh.RejectCuti)

	mux.HandleFunc("POST /leave/sakit/request", lh.RequestSakit)
	mux.HandleFunc("POST /leave/sakit/{id}/approve", lh.ApproveSakit)
	mux.HandleFunc("POST /leave/sakit/{id}/reject", lh.RejectSakit)
	mux.HandleFunc("GET /leave/sakit/list", lh.ListSakit)

	return mux
}
