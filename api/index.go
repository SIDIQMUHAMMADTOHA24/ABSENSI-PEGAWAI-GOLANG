// api/index.go
package handler

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

var once sync.Once
var app http.Handler
var sqlDB *sql.DB

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

	// serverless friendly pool settings
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	app = router.New(sqlDB)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	app.ServeHTTP(w, r)
}
