// api/index.go
package handler

import (
	"database/sql"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"absensi/app" // <— pakai wrapper publik
	"absensi/internal/db"
)

var (
	once  sync.Once
	srv   http.Handler
	sqlDB *sql.DB
)

func initApp() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		panic("DATABASE_URL is not set")
	}

	var err error
	sqlDB, err = db.Connect(dsn)
	if err != nil {
		panic(err)
	}

	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	srv = app.NewHandler(sqlDB) // <— panggil wrapper, bukan router langsung
}

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	srv.ServeHTTP(w, r)
}
