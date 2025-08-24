package app

import (
	"absensi/internal/db"
	"absensi/internal/http/router"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
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
		return nil, fmt.Errorf("connect db: %w", err)
	}

	// tuning serverless
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return &Server{
		DB:      sqlDB,
		Handler: router.New(sqlDB),
	}, nil
}
