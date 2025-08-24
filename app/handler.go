// app/handler.go
package app

import (
	"database/sql"
	"net/http"

	"absensi/internal/http/router"
)

func NewHandler(db *sql.DB) http.Handler {
	return router.New(db)
}
