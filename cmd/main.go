package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Jabatan  string `json:"jabatan"`
}

type UserDTO struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Jabatan   string    `json:"jabatan"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {

	// Contoh DSN, sesuaikan kredensial & nama DB (absesnsi)
	// Format pgx stdlib: "postgres://user:pass@host:port/dbname?sslmode=disable"
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/absesnsi?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// optional: cek koneksi
	if err := db.Ping(); err != nil {
		log.Fatal("cannot connect db: ", err)
	}

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		// Validasi sederhana
		if len(req.Username) < 3 || len(req.Password) < 6 || req.Jabatan == "" {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		var u UserDTO
		// Insert + RETURNING
		q := `
			INSERT INTO users (username, password_hash, jabatan)
			VALUES ($1, $2, $3)
			RETURNING id::text, username, jabatan, created_at;
		`
		err = db.QueryRowContext(ctx, q, req.Username, string(hash), req.Jabatan).
			Scan(&u.ID, &u.Username, &u.Jabatan, &u.CreatedAt)

		if err != nil {
			if contains(err.Error(), "duplicate key") || contains(err.Error(), "unique constraint") {
				http.Error(w, "username already exists", http.StatusConflict)
				return
			}
			http.Error(w, "failed to register user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"message": "registered",
			"user":    u,
		})
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// helper kecil (biar gak import strings berlebihan)
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
