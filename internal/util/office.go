package util

import (
	"math"
	"time"
)

// Hardcode dulu; nanti bisa pindah ke env/DB
var (
	OfficeLat     = -7.688260
	OfficeLng     = 110.187048
	OfficeRadiusM = 20.0
	epsilonM      = 5.0 // toleransi GPS
)

var OfficeTZ = func() *time.Location {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	return loc
}()

// OfficeDate: tanggal di timezone kantor
func OfficeDate(t time.Time) time.Time {
	local := t.In(OfficeTZ)
	// kembalikan jam 00:00 lokal sebagai anchor (DATE)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, OfficeTZ)
}

// Haversine meters
func HaversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	toRad := func(d float64) float64 { return d * 3.141592653589793 / 180.0 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	lat1r := toRad(lat1)
	lat2r := toRad(lat2)
	a := (sin(dLat/2)*sin(dLat/2) +
		cos(lat1r)*cos(lat2r)*sin(dLon/2)*sin(dLon/2))
	c := 2 * atan2Sqrt(a)
	return R * c
}

func sin(x float64) float64 { return float64From(mathSin(float64(x))) }
func cos(x float64) float64 { return float64From(mathCos(float64(x))) }
func atan2Sqrt(a float64) float64 {
	return float64From(mathAtan2(mathSqrt(float64(a)), mathSqrt(float64(1-a))))
}

// gunakan math langsung (ditulis begini supaya jelas tidak perlu import alias)

func float64From(f float64) float64  { return f }
func mathSin(x float64) float64      { return math.Sin(x) }
func mathCos(x float64) float64      { return math.Cos(x) }
func mathAtan2(y, x float64) float64 { return math.Atan2(y, x) }
func mathSqrt(x float64) float64     { return math.Sqrt(x) }

// inside radius dengan toleransi
func InsideRadius(distance float64) bool {
	return distance <= (OfficeRadiusM + epsilonM)
}
