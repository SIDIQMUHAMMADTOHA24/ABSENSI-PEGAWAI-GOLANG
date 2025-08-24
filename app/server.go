package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"absensi/internal/db"
	"absensi/internal/http/router"
)

type Server struct {
	DB      *sql.DB
	Handler http.Handler
}

func NewFromEnv() (*Server, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	sqlDB, err := db.Connect(dsn)
	if err != nil {
		return nil, err
	}

	// Tuning kecil biar aman di serverless
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return &Server{
		DB:      sqlDB,
		Handler: router.New(sqlDB),
	}, nil
}
