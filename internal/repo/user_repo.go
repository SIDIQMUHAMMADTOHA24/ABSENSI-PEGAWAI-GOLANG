package repo

import (
	"context"
	"database/sql"
	"time"

	"absensi/internal/models"
)

type UserRepo struct{ DB *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{DB: db} }

func (r *UserRepo) Create(ctx context.Context, username, passHash, jabatan string) (models.User, error) {
	q := `INSERT INTO users (username, password_hash, jabatan)
	      VALUES ($1,$2,$3)
	      RETURNING id::text, username, jabatan, created_at;`
	var u models.User
	err := r.DB.QueryRowContext(ctx, q, username, passHash, jabatan).
		Scan(&u.ID, &u.Username, &u.Jabatan, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (models.User, error) {
	q := `SELECT id::text, username, password_hash, jabatan, created_at
	      FROM users WHERE username=$1;`
	var u models.User
	err := r.DB.QueryRowContext(ctx, q, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Jabatan, &u.CreatedAt)
	return u, err
}

type RefreshRepo struct{ DB *sql.DB }

func NewRefreshRepo(db *sql.DB) *RefreshRepo { return &RefreshRepo{DB: db} }

func (r *RefreshRepo) Store(ctx context.Context, userID, token string, exp time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1,$2,$3)`,
		userID, token, exp)
	return err
}

func (r *RefreshRepo) Revoke(ctx context.Context, token string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked=TRUE WHERE token=$1`, token)
	return err
}

func (r *RefreshRepo) IsValid(ctx context.Context, token string, now time.Time) (string, bool, error) {
	var userID string
	var revoked bool
	var exp time.Time
	err := r.DB.QueryRowContext(ctx,
		`SELECT user_id, revoked, expires_at FROM refresh_tokens WHERE token=$1`, token).
		Scan(&userID, &revoked, &exp)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	if revoked || now.After(exp) {
		return "", false, nil
	}
	return userID, true, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (models.User, error) {
	q := `SELECT id::text, username, password_hash, jabatan, created_at FROM users WHERE id=$1;`
	var u models.User
	err := r.DB.QueryRowContext(ctx, q, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Jabatan, &u.CreatedAt)
	return u, err
}
