package service

import (
	"errors"
	"math/big"
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
	expectedEnd := expectedStart.Add(7 * 24 * time.Hour)
	if !info.WeekStartDate.Equal(expectedStart) || !info.WeekEndDate.Equal(expectedEnd) {
		t.Fatalf("unexpected week window: [%s, %s)", info.WeekStartDate, info.WeekEndDate)
	}
	if info.NodeEmissionPoolCNX != 9280212 {
		t.Fatalf("unexpected node emission pool: %d", info.NodeEmissionPoolCNX)
	}
}

func TestGetCurrentEmissionWeekInfoReturnsCurrentIncompleteWeek(t *testing.T) {
	start := "2026-01-01T09:33:00+08:00"
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	info, err := GetCurrentEmissionWeekInfo(now, start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info.WeekIndex != 1 || info.YearIndex != 1 {
		t.Fatalf("unexpected week/year: week=%d year=%d", info.WeekIndex, info.YearIndex)
	}
	expectedStart := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	expectedEnd := expectedStart.Add(7 * 24 * time.Hour)
	if !info.WeekStartDate.Equal(expectedStart) || !info.WeekEndDate.Equal(expectedEnd) {
		t.Fatalf("unexpected week window: [%s, %s)", info.WeekStartDate, info.WeekEndDate)
	}
	if info.NodeEmissionPoolCNX != 9280212 {
		t.Fatalf("unexpected node emission pool: %d", info.NodeEmissionPoolCNX)
	}
}

func TestGetCurrentEmissionWeekInfoUsesFirstWeekBeforeMainnetStart(t *testing.T) {
	start := "2026-01-01T00:00:00Z"
	now := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)

	info, err := GetCurrentEmissionWeekInfo(now, start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info.WeekIndex != 0 || !info.WeekStartDate.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected current week: index=%d start=%s", info.WeekIndex, info.WeekStartDate)
	}
}

func TestGetPreviousEmissionWeekInfoRequiresCompletedWeek(t *testing.T) {
	start := "2026-01-01T00:00:00Z"
	now := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

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

func TestNormalizeToUTCWeekStart(t *testing.T) {
	ts := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)
	normalized := NormalizeToUTCWeekStart(ts)
	expected := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
	if !normalized.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, normalized)
	}
}

func TestBuildEmissionChartRangeReturns24StartTimeBuckets(t *testing.T) {
	mainnetStart := "2026-01-01T09:33:00+08:00"
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	chartRange, err := BuildEmissionChartRange(now, mainnetStart, 24)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(chartRange.WeekStarts) != 24 {
		t.Fatalf("expected 24 week points, got %d", len(chartRange.WeekStarts))
	}
	if !chartRange.RangeEnd.After(chartRange.RangeStart) {
		t.Fatalf("expected range end > range start, got [%s, %s)", chartRange.RangeStart, chartRange.RangeEnd)
	}
	if !chartRange.WeekStarts[0].Equal(chartRange.RangeStart) {
		t.Fatalf("first point must equal range start: first=%s range_start=%s", chartRange.WeekStarts[0], chartRange.RangeStart)
	}
	last := chartRange.WeekStarts[len(chartRange.WeekStarts)-1]
	if !last.Equal(chartRange.RangeEnd.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("last point must be one week before range end: last=%s range_end=%s", last, chartRange.RangeEnd)
	}
}

func TestBuildEmissionChartRangeIncludesCurrentStartTimeBucket(t *testing.T) {
	mainnetStart := "2026-01-01T00:00:00Z"
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	chartRange, err := BuildEmissionChartRange(now, mainnetStart, 24)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(chartRange.WeekStarts) != 24 {
		t.Fatalf("expected 24 week points, got %d", len(chartRange.WeekStarts))
	}
	expectedEnd := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !chartRange.RangeEnd.Equal(expectedEnd) {
		t.Fatalf("expected range end %s, got %s", expectedEnd, chartRange.RangeEnd)
	}
	last := chartRange.WeekStarts[len(chartRange.WeekStarts)-1]
	if !last.Equal(time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected last point at current week start, got %s", last)
	}
}

func TestBuildEmissionChartRangeReturnsRequestedPointsBeforeFirstWeekCompletes(t *testing.T) {
	mainnetStart := "2026-01-01T00:00:00Z"
	now := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

	chartRange, err := BuildEmissionChartRange(now, mainnetStart, 24)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(chartRange.WeekStarts) != 24 {
		t.Fatalf("expected 24 week points, got %d", len(chartRange.WeekStarts))
	}
	if !chartRange.RangeEnd.Equal(time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected range end at next week boundary, got %s", chartRange.RangeEnd)
	}
	last := chartRange.WeekStarts[len(chartRange.WeekStarts)-1]
	if !last.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected last point at mainnet start, got %s", last)
	}
}

func TestAlignToMainnetEmissionWeekStart(t *testing.T) {
	mainnetWeekStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recordStart := time.Date(2026, 1, 6, 18, 0, 0, 0, time.UTC)

	aligned, ok := AlignToMainnetEmissionWeekStart(recordStart, mainnetWeekStart)
	if !ok {
		t.Fatal("expected record to map to a valid mainnet-aligned week")
	}
	expected := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !aligned.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, aligned)
	}
}

func TestParseMainnetAlignedWeekStartCutsToUTCDateStart(t *testing.T) {
	aligned, err := ParseMainnetAlignedWeekStart("2026-01-01T09:33:00+08:00")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	expected := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !aligned.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, aligned)
	}
}

func TestBuildEmissionChartRangeUsesStartTimeSevenDayWeeks(t *testing.T) {
	mainnetStart := "2026-06-01T00:00:00Z"
	now := time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC)

	chartRange, err := BuildEmissionChartRange(now, mainnetStart, 24)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(chartRange.WeekStarts) != 24 {
		t.Fatalf("expected 24 week points, got %d", len(chartRange.WeekStarts))
	}
	expectedEnd := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if !chartRange.RangeEnd.Equal(expectedEnd) {
		t.Fatalf("expected next week boundary %s, got %s", expectedEnd, chartRange.RangeEnd)
	}
	last := chartRange.WeekStarts[len(chartRange.WeekStarts)-1]
	if !last.Equal(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected last point at current week start, got %s", last)
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

func TestGetCNXTotalSupply(t *testing.T) {
	expected := cnxToWei(cnxTotalSupplyCNX)

	totalSupply := GetCNXTotalSupply()
	if totalSupply.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, totalSupply)
	}
}

func TestGetCNXCirculatingSupplyReturnsZeroBeforeMainnet(t *testing.T) {
	supply, err := GetCNXCirculatingSupply(
		time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		"2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if supply.Sign() != 0 {
		t.Fatalf("expected zero supply, got %s", supply)
	}
}

func TestGetCNXCirculatingSupplyAtMainnetStartIncludesUnlockedYear0(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	supply, err := GetCNXCirculatingSupply(now, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	year0NodeVestingCNX := int64(year0EmissionCNX * year0NodeAllocationPercent / 100)
	expected := cnxToWei(year0EmissionCNX - year0NodeVestingCNX)
	if supply.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, supply)
	}
}

func TestGetCNXCirculatingSupplyIncludesReleasedVestingAfterFirstWeek(t *testing.T) {
	mainnetStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	supply, err := GetCNXCirculatingSupply(now, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	year0NodeVestingCNX := int64(year0EmissionCNX * year0NodeAllocationPercent / 100)
	year0Unlocked := cnxToWei(year0EmissionCNX - year0NodeVestingCNX)
	year0Released := releasedVestingAmount(
		cnxToWei(year0NodeVestingCNX),
		mainnetStart,
		year0VestingDurationDays,
		now,
	)

	weeklyEmissionCNX := weeklyEmissionCNXByYear[0]
	weeklyNodeVestingCNX := weeklyEmissionCNX * nodeAllocationPercent(1) / 100
	weeklyUnlocked := cnxToWei(weeklyEmissionCNX - weeklyNodeVestingCNX)
	weeklyReleased := releasedVestingAmount(
		cnxToWei(weeklyNodeVestingCNX),
		mainnetStart.Add(emissionWeekDuration),
		nodeVestingDurationDays,
		now,
	)

	expected := big.NewInt(0)
	expected.Add(expected, year0Unlocked)
	expected.Add(expected, year0Released)
	expected.Add(expected, weeklyUnlocked)
	expected.Add(expected, weeklyReleased)

	if supply.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, supply)
	}
}
