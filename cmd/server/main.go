package main

import (
	"log"
	"net/http"
	"os"

	"absensi/internal/db"
	router "absensi/internal/http/router"

	"github.com/joho/godotenv"
)

func main() {
	wd, _ := os.Getwd()
	log.Println("cwd:", wd)
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	sqlDB, err := db.Connect(dsn)
	if err != nil {
		log.Fatal("connect db:", err)
	}
	defer sqlDB.Close()

	mux := router.New(sqlDB)
	addr := ":8080"
	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
