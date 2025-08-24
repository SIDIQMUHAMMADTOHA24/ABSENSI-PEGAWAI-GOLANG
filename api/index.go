// api/index.go
package main

import (
	"database/sql"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"absensi/internal/db"
	"absensi/internal/http/router"
)

var (
	once  sync.Once
	app   http.Handler
	sqlDB *sql.DB
)

func initApp() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Di Vercel, pakai env di dashboard/CLI, jangan .env file
		panic("DATABASE_URL is not set")
	}
	// koneksi pool kecil supaya ramah serverless
	var err error
	sqlDB, err = db.Connect(dsn)
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	app = router.New(sqlDB) // router kamu sudah return http.Handler
}

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	// Semua route /api/* bakal jatuh ke mux milik kamu
	app.ServeHTTP(w, r)
}
