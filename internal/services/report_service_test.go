package services_test

import (
	"testing"
	"time"

	"wedrink/internal/utils"
)

func TestPakistanTimezoneAndBoundaries(t *testing.T) {
	pkToday := utils.PKTodayStr()
	if len(pkToday) != 10 {
		t.Errorf("Expected YYYY-MM-DD format for PKTodayStr, got: %s", pkToday)
	}

	// July 1, 2026 lower boundary
	if !utils.IsBeforeMinDate("2026-06-30") {
		t.Errorf("Expected 2026-06-30 to be before min date %s", utils.MinDateStr)
	}
	if utils.IsBeforeMinDate("2026-07-01") {
		t.Errorf("Expected 2026-07-01 to NOT be before min date %s", utils.MinDateStr)
	}

	// Future date test
	futureDate := utils.PKNow().AddDate(0, 0, 2).Format("2006-01-02")
	if !utils.IsFutureDate(futureDate) {
		t.Errorf("Expected %s to be recognized as future date", futureDate)
	}

	pastValidDate := "2026-07-05"
	if utils.IsBeforeMinDate(pastValidDate) {
		t.Errorf("Expected %s to be valid past date", pastValidDate)
	}

	loc := utils.PKLocation
	if loc == nil {
		t.Fatalf("PKLocation should not be nil")
	}
	nowInLoc := time.Now().In(loc)
	if nowInLoc.Location().String() != "Asia/Karachi" && nowInLoc.Location().String() != "PKT" {
		t.Errorf("Unexpected location: %s", nowInLoc.Location().String())
	}
}
