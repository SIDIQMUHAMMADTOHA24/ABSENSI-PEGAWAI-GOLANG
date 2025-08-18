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

	return mux
}
