package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// --- Tambahan: model minimal untuk ambil 1 pengajuan ---
type LeaveRequest struct {
	ID        string
	UserID    string
	Kind      string
	Status    string
	Reason    sql.NullString
	StartDate time.Time
	EndDate   time.Time
	Days      int
	CreatedAt time.Time
	DecidedAt sql.NullTime
}

// Ambil 1 pengajuan berdasarkan ID
func (r *LeaveRepo) GetLeaveByID(ctx context.Context, id string) (LeaveRequest, error) {
	const q = `
		SELECT id::text, user_id::text, kind, status, reason,
		       start_date, end_date, days, created_at, decided_at
		FROM leave_requests
		WHERE id = $1
		LIMIT 1;
	`
	var lr LeaveRequest
	err := r.DB.QueryRowContext(ctx, q, id).
		Scan(&lr.ID, &lr.UserID, &lr.Kind, &lr.Status, &lr.Reason,
			&lr.StartDate, &lr.EndDate, &lr.Days, &lr.CreatedAt, &lr.DecidedAt)
	if err == sql.ErrNoRows {
		return LeaveRequest{}, nil
	}
	return lr, err
}

// Set status -> approved (hanya jika masih pending)
func (r *LeaveRepo) ApproveCutiPending(ctx context.Context, id string, decidedAt time.Time) (bool, error) {
	const q = `
		UPDATE leave_requests
		SET status = 'approved', decided_at = $2
		WHERE id = $1 AND status = 'pending' AND kind = 'cuti'
	`
	res, err := r.DB.ExecContext(ctx, q, id, decidedAt)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Set status -> rejected (hanya jika masih pending)
func (r *LeaveRepo) RejectCutiPending(ctx context.Context, id string, decidedAt time.Time) (bool, error) {
	const q = `
		UPDATE leave_requests
		SET status = 'rejected', decided_at = $2
		WHERE id = $1 AND status = 'pending' AND kind = 'cuti'
	`
	res, err := r.DB.ExecContext(ctx, q, id, decidedAt)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

type LeaveRepo struct{ DB *sql.DB }

func NewLeaveRepo(db *sql.DB) *LeaveRepo { return &LeaveRepo{DB: db} }

// SumApprovedDays: jumlah hari cuti (approved) dalam 1 tahun kalender.
func (r *LeaveRepo) SumApprovedDays(ctx context.Context, userID string, year int) (int, error) {
	const q = `
		SELECT COALESCE(SUM(days),0)
		FROM leave_requests
		WHERE user_id = $1
		  AND kind = 'cuti'
		  AND status = 'approved'
		  AND EXTRACT(YEAR FROM start_date) = $2
	`
	var total int
	if err := r.DB.QueryRowContext(ctx, q, userID, year).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// CreateCutiPending: insert pengajuan cuti status pending.
func (r *LeaveRepo) CreateCutiPending(
	ctx context.Context,
	userID string,
	start, end time.Time,
	days int,
	reason string,
) (string, error) {
	const q = `
		INSERT INTO leave_requests (user_id, kind, status, reason, start_date, end_date, days)
		VALUES ($1, 'cuti', 'pending', $2, $3::date, $4::date, $5)
		RETURNING id::text
	`
	var id string
	if err := r.DB.QueryRowContext(ctx, q, userID, reason,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
		days,
	).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// (opsional) Validasi overlap kalau nanti dibutuhkan:
// cek bentrok dengan pengajuan lain (pending/approved) pada rentang tanggal yang sama.
func (r *LeaveRepo) HasOverlap(ctx context.Context, userID string, start, end time.Time) (bool, error) {
	const q = `
		SELECT EXISTS (
		  SELECT 1
		  FROM leave_requests
		  WHERE user_id = $1
		    AND kind = 'cuti'
		    AND status IN ('pending','approved')
		    AND NOT ($4::date < start_date OR $3::date > end_date)
		)
	`
	var exists bool
	err := r.DB.QueryRowContext(ctx, q, userID,
		start.Format("2006-01-02"),
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	).Scan(&exists)
	return exists, err
}

var ErrInvalidRange = errors.New("invalid date range")

func (r *LeaveRepo) ListCutiByYearStatus(
	ctx context.Context,
	userID string,
	year int,
	status string,
) ([]LeaveRequest, error) {

	base := `
		SELECT id::text, user_id::text, kind, status, reason,
		       start_date, end_date, days, created_at, decided_at
		FROM leave_requests
		WHERE user_id = $1
		  AND kind = 'cuti'
		  AND EXTRACT(YEAR FROM start_date) = $2
	`
	var rows *sql.Rows
	var err error

	switch status {
	case "", "all":
		q := base + ` ORDER BY start_date DESC, created_at DESC`
		rows, err = r.DB.QueryContext(ctx, q, userID, year)
	default:
		q := base + ` AND status = $3 ORDER BY start_date DESC, created_at DESC`
		rows, err = r.DB.QueryContext(ctx, q, userID, year, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LeaveRequest
	for rows.Next() {
		var lr LeaveRequest
		if err := rows.Scan(
			&lr.ID, &lr.UserID, &lr.Kind, &lr.Status, &lr.Reason,
			&lr.StartDate, &lr.EndDate, &lr.Days, &lr.CreatedAt, &lr.DecidedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, lr)
	}
	return out, rows.Err()
}
