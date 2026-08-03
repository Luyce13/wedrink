package utils

import (
	"log/slog"
	"time"
)

const MinDateStr = "2026-07-01"

var PKLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Karachi")
	if err != nil {
		slog.Warn("Could not load Asia/Karachi timezone location, using fixed UTC+5 offset", "error", err)
		loc = time.FixedZone("PKT", 5*3600)
	}
	PKLocation = loc
}

// PKNow returns current time in Pakistan timezone (Asia/Karachi).
func PKNow() time.Time {
	return time.Now().In(PKLocation)
}

// PKTodayStr returns today's date formatted as YYYY-MM-DD in Pakistan timezone.
func PKTodayStr() string {
	return PKNow().Format("2006-01-02")
}

// PKYesterdayStr returns yesterday's date formatted as YYYY-MM-DD in Pakistan timezone.
func PKYesterdayStr() string {
	return PKNow().AddDate(0, 0, -1).Format("2006-01-02")
}

// IsFutureDate checks if dateStr (YYYY-MM-DD) is after today in Pakistan timezone.
func IsFutureDate(dateStr string) bool {
	return dateStr > PKTodayStr()
}

// IsBeforeMinDate checks if dateStr (YYYY-MM-DD) is prior to July 1, 2026.
func IsBeforeMinDate(dateStr string) bool {
	return dateStr < MinDateStr
}
