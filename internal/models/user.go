package models

import "time"

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Jabatan      string
	CreatedAt    time.Time
}
