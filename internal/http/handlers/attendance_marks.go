package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// ===== GET /attendance/marks?month=YYYY-MM&tz=Asia/Jakarta

type marksResp struct {
	Month       string   `json:"month"`
	DaysPresent []string `json:"days_present"`
}

func (h *AttendanceHandler) GetMarks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	month := q.Get("month") // ex: 2025-08
	tz := q.Get("tz")
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		http.Error(w, "invalid tz", http.StatusBadRequest)
		return
	}

	// Resolve month start/end
	var start time.Time
	if month == "" {
		now := time.Now().In(loc)
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	} else {
		start, err = time.ParseInLocation("2006-01", month, loc)
		if err != nil {
			http.Error(w, "invalid month", http.StatusBadRequest)
			return
		}
	}
	end := start.AddDate(0, 1, 0)

	// Ambil user ID (sesuaikan dengan auth kamu)
	uid, ok := userIDFromRequest(r)
	if !ok || uid == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dates, err := h.Attendance.ListMarkedDays(ctx, uid, start, end)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := marksResp{
		Month:       start.Format("2006-01"),
		DaysPresent: make([]string, 0, len(dates)),
	}
	for _, d := range dates {
		out.DaysPresent = append(out.DaysPresent, d.Format("2006-01-02"))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ===== GET /attendance/day?date=YYYY-MM-DD&tz=Asia/Jakarta

type dayEvent struct {
	Type        string   `json:"type"`
	At          string   `json:"at"`
	Lat         *float64 `json:"lat,omitempty"`
	Lng         *float64 `json:"lng,omitempty"`
	DistanceM   *float64 `json:"distance_m,omitempty"`
	PhotoBase64 *string  `json:"photo_base64,omitempty"`
}
type dayResp struct {
	Date          string     `json:"date"`
	Events        []dayEvent `json:"events"`
	WorkedSeconds int64      `json:"worked_seconds"`
}

func (h *AttendanceHandler) GetDay(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dateStr := q.Get("date") // required
	if dateStr == "" {
		http.Error(w, "missing date", http.StatusBadRequest)
		return
	}
	tz := q.Get("tz")
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		http.Error(w, "invalid tz", http.StatusBadRequest)
		return
	}
	day, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	uid, ok := userIDFromRequest(r)
	if !ok || uid == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	raw, err := h.Attendance.GetDayRaw(ctx, uid, day)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	events := make([]dayEvent, 0, 2)
	var worked int64

	// Helper to take pointer if valid
	ptr := func(n sql.NullFloat64) *float64 {
		if n.Valid {
			v := n.Float64
			return &v
		}
		return nil
	}

	ptrS := func(s sql.NullString) *string {
		if s.Valid {
			v := s.String
			return &v
		}
		return nil
	}

	if raw.CheckInAt.Valid {
		events = append(events, dayEvent{
			Type:        "check_in",
			At:          raw.CheckInAt.Time.UTC().Format(time.RFC3339),
			Lat:         ptr(raw.InLat),
			Lng:         ptr(raw.InLng),
			DistanceM:   ptr(raw.InDist),
			PhotoBase64: ptrS(raw.InPhotoB64),
		})
	}
	if raw.CheckOutAt.Valid {
		events = append(events, dayEvent{
			Type:        "check_out",
			At:          raw.CheckOutAt.Time.UTC().Format(time.RFC3339),
			Lat:         ptr(raw.OutLat),
			Lng:         ptr(raw.OutLng),
			DistanceM:   ptr(raw.OutDist),
			PhotoBase64: ptrS(raw.OutPhotoB64),
		})
	}
	if raw.CheckInAt.Valid && raw.CheckOutAt.Valid {
		worked = int64(raw.CheckOutAt.Time.Sub(raw.CheckInAt.Time).Seconds())
	}

	resp := dayResp{
		Date:          day.Format("2006-01-02"),
		Events:        events,
		WorkedSeconds: worked,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
