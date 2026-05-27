package service

import (
	"errors"
	"testing"
	"time"
)

func TestGetPreviousEmissionWeekInfoRejectsMissingStartTime(t *testing.T) {
	_, err := GetPreviousEmissionWeekInfo(time.Now().UTC(), "")
	if !errors.Is(err, ErrMainnetStartTimeNotSet) {
		t.Fatalf("expected ErrMainnetStartTimeNotSet, got %v", err)
	}
}

func TestGetPreviousEmissionWeekInfoRejectsInvalidStartTime(t *testing.T) {
	_, err := GetPreviousEmissionWeekInfo(time.Now().UTC(), "2026/01/01")
	if !errors.Is(err, ErrInvalidMainnetStart) {
		t.Fatalf("expected ErrInvalidMainnetStart, got %v", err)
	}
}

func TestGetPreviousEmissionWeekInfoReturnsPreviousCompleteWeek(t *testing.T) {
	start := "2026-01-01T09:33:00+08:00"
	now := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)

	info, err := GetPreviousEmissionWeekInfo(now, start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info.WeekIndex != 0 || info.YearIndex != 1 {
		t.Fatalf("unexpected week/year: week=%d year=%d", info.WeekIndex, info.YearIndex)
	}
	expectedStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedEnd := expectedStart.AddDate(0, 0, 7)
	if !info.WeekStartDate.Equal(expectedStart) || !info.WeekEndDate.Equal(expectedEnd) {
		t.Fatalf("unexpected week window: [%s, %s)", info.WeekStartDate, info.WeekEndDate)
	}
	if info.NodeEmissionPoolCNX != 9280212 {
		t.Fatalf("unexpected node emission pool: %d", info.NodeEmissionPoolCNX)
	}
}

func TestGetPreviousEmissionWeekInfoRequiresCompletedWeek(t *testing.T) {
	start := "2026-01-01T00:00:00Z"
	now := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)

	_, err := GetPreviousEmissionWeekInfo(now, start)
	if !errors.Is(err, ErrNoCompletedEmissionWeek) {
		t.Fatalf("expected ErrNoCompletedEmissionWeek, got %v", err)
	}
}

func TestGetPreviousEmissionWeekInfoMapsYear2Allocation(t *testing.T) {
	start := "2026-01-01T00:00:00Z"
	now := time.Date(2027, 1, 7, 0, 0, 0, 0, time.UTC)

	info, err := GetPreviousEmissionWeekInfo(now, start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info.WeekIndex != 52 || info.YearIndex != 2 {
		t.Fatalf("unexpected week/year: week=%d year=%d", info.WeekIndex, info.YearIndex)
	}
	if info.NodeEmissionPoolCNX != 9737416 {
		t.Fatalf("unexpected node emission pool: %d", info.NodeEmissionPoolCNX)
	}
}

func TestGetPreviousEmissionWeekInfoRejectsOutOfRangeYear(t *testing.T) {
	start := "2026-01-01T00:00:00Z"
	now := time.Date(2045, 12, 31, 0, 0, 0, 0, time.UTC)

	_, err := GetPreviousEmissionWeekInfo(now, start)
	if !errors.Is(err, ErrEmissionWeekOutOfRange) {
		t.Fatalf("expected ErrEmissionWeekOutOfRange, got %v", err)
	}
}
