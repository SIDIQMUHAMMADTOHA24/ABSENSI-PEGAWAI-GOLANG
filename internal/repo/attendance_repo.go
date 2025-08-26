package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AttendanceRepo struct{ DB *sql.DB }

func NewAttendanceRepo(db *sql.DB) *AttendanceRepo { return &AttendanceRepo{DB: db} }

type AttendanceDay struct {
	ID            string
	UserID        string
	Date          time.Time // anchor 00:00 lokal, tapi simpan sebagai DATE di DB
	CheckInAt     sql.NullTime
	CheckOutAt    sql.NullTime
	WorkedSeconds int64
}

func (r *AttendanceRepo) GetByUserAndDate(ctx context.Context, userID string, date time.Time) (AttendanceDay, error) {
	q := `
	SELECT id::text, user_id, date,
	       check_in_at, check_out_at
	FROM attendance_days
	WHERE user_id=$1 AND date=$2::date
	`
	var ad AttendanceDay
	err := r.DB.QueryRowContext(ctx, q, userID, date.Format("2006-01-02")).
		Scan(&ad.ID, &ad.UserID, &ad.Date, &ad.CheckInAt, &ad.CheckOutAt)
	if err == sql.ErrNoRows {
		return AttendanceDay{}, nil
	}
	if err != nil {
		return AttendanceDay{}, err
	}
	// hitung worked_seconds kalau ada check-out
	if ad.CheckInAt.Valid && ad.CheckOutAt.Valid {
		ad.WorkedSeconds = int64(ad.CheckOutAt.Time.Sub(ad.CheckInAt.Time).Seconds())
	}
	return ad, nil
}

// Insert check-in jika belum ada; kalau baris sudah ada dan check_in_at NULL → isi sekarang.
func (r *AttendanceRepo) DoCheckIn(ctx context.Context, userID string, date time.Time, now time.Time, lat, lng, dist float64) (AttendanceDay, error) {
	q := `
	INSERT INTO attendance_days (user_id, date, check_in_at, check_in_lat, check_in_lng, check_in_distance_m)
	VALUES ($1, $2::date, $3, $4, $5, $6)
	ON CONFLICT (user_id, date)
	DO UPDATE SET
		check_in_at = COALESCE(attendance_days.check_in_at, EXCLUDED.check_in_at),
		check_in_lat = COALESCE(attendance_days.check_in_lat, EXCLUDED.check_in_lat),
		check_in_lng = COALESCE(attendance_days.check_in_lng, EXCLUDED.check_in_lng),
		check_in_distance_m = COALESCE(attendance_days.check_in_distance_m, EXCLUDED.check_in_distance_m),
		updated_at = NOW()
	WHERE attendance_days.check_in_at IS NULL
	RETURNING id::text, user_id, date, check_in_at, check_out_at
	`
	var ad AttendanceDay
	err := r.DB.QueryRowContext(ctx, q, userID, date.Format("2006-01-02"), now, lat, lng, dist).
		Scan(&ad.ID, &ad.UserID, &ad.Date, &ad.CheckInAt, &ad.CheckOutAt)
	return ad, err
}

// Update check-out jika sudah check-in dan check-out masih NULL
func (r *AttendanceRepo) DoCheckOut(ctx context.Context, userID string, date time.Time, now time.Time, lat, lng, dist float64) (AttendanceDay, error) {
	q := `
	UPDATE attendance_days
	SET check_out_at=$3, check_out_lat=$4, check_out_lng=$5, check_out_distance_m=$6, updated_at=NOW()
	WHERE user_id=$1 AND date=$2::date AND check_in_at IS NOT NULL AND check_out_at IS NULL
	RETURNING id::text, user_id, date, check_in_at, check_out_at
	`
	var ad AttendanceDay
	err := r.DB.QueryRowContext(ctx, q, userID, date.Format("2006-01-02"), now, lat, lng, dist).
		Scan(&ad.ID, &ad.UserID, &ad.Date, &ad.CheckInAt, &ad.CheckOutAt)
	return ad, err
}

func (r *AttendanceRepo) ResetToday(ctx context.Context, userID, yyyymmdd string) (int64, error) {
	res, err := r.DB.ExecContext(ctx, `
		DELETE FROM attendance_days
		WHERE user_id = $1 AND date = $2::date
	`, userID, yyyymmdd)
	if err != nil {
		return 0, fmt.Errorf("delete attendance_days: %w", err)
	}
	return res.RowsAffected()
}

func (r *AttendanceRepo) ListMarkedDays(ctx context.Context, userID string, from, to time.Time) ([]time.Time, error) {
	const q = `
		SELECT date
		FROM attendance_days
		WHERE user_id = $1
		  AND date >= $2::date
		  AND date <  $3::date
		  AND (check_in_at IS NOT NULL OR check_out_at IS NOT NULL)
		ORDER BY date;
	`
	rows, err := r.DB.QueryContext(ctx, q,
		userID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDayRaw: ambil detail in/out untuk 1 hari.
type DayRaw struct {
	CheckInAt  sql.NullTime
	InLat      sql.NullFloat64
	InLng      sql.NullFloat64
	InDist     sql.NullFloat64
	CheckOutAt sql.NullTime
	OutLat     sql.NullFloat64
	OutLng     sql.NullFloat64
	OutDist    sql.NullFloat64
}

func (r *AttendanceRepo) GetDayRaw(ctx context.Context, userID string, date time.Time) (DayRaw, error) {
	const q = `
		SELECT
			check_in_at,  check_in_lat,  check_in_lng,  check_in_distance_m,
			check_out_at, check_out_lat, check_out_lng, check_out_distance_m
		FROM attendance_days
		WHERE user_id = $1 AND date = $2::date
		LIMIT 1;
	`
	var dr DayRaw
	err := r.DB.QueryRowContext(ctx, q,
		userID,
		date.Format("2006-01-02"),
	).Scan(
		&dr.CheckInAt, &dr.InLat, &dr.InLng, &dr.InDist,
		&dr.CheckOutAt, &dr.OutLat, &dr.OutLng, &dr.OutDist,
	)
	if err == sql.ErrNoRows {
		return DayRaw{}, nil
	}
	return dr, err
}
