package util

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func mustEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func AccessTokenTTL() time.Duration {
	minStr := mustEnv("ACCESS_TOKEN_TTL_MIN", "15")
	min, _ := strconv.Atoi(minStr)
	return time.Duration(min) * time.Minute
}

func RefreshTokenTTL() time.Duration {
	dayStr := mustEnv("REFRESH_TOKEN_TTL_DAY", "7")
	day, _ := strconv.Atoi(dayStr)
	return time.Duration(day) * 24 * time.Hour
}

func SignAccessToken(userID, username string) (string, time.Time, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret" // ganti di produksi
	}
	now := time.Now()
	exp := now.Add(AccessTokenTTL())
	claims := jwt.MapClaims{
		"sub": userID,
		"usr": username,
		"iat": now.Unix(),
		"exp": exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, exp, err
}

func ParseAccessToken(tokenStr string) (userID, username string, err error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return "", "", errors.New("invalid token")
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid claims")
	}
	sub, _ := claims["sub"].(string)
	usr, _ := claims["usr"].(string)
	if sub == "" {
		return "", "", errors.New("no sub")
	}
	return sub, usr, nil
}
