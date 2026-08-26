package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newEmissionSupplyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open supply test database: %v", err)
	}
	if err := db.AutoMigrate(&models.VestingRecord{}); err != nil {
		t.Fatalf("migrate supply test database: %v", err)
	}
	return db
}

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
	start := "2026-06-17T00:00:00Z"
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)

	info, err := GetPreviousEmissionWeekInfo(now, start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info.WeekIndex != 9 || info.YearIndex != 1 {
		t.Fatalf("unexpected week/year: week=%d year=%d", info.WeekIndex, info.YearIndex)
	}
	expectedStart := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	expectedEnd := expectedStart.Add(7 * 24 * time.Hour)
	if !info.WeekStartDate.Equal(expectedStart) || !info.WeekEndDate.Equal(expectedEnd) {
		t.Fatalf("unexpected week window: [%s, %s)", info.WeekStartDate, info.WeekEndDate)
	}
	if info.NodeEmissionPoolCNX != 1350649 {
		t.Fatalf("unexpected node emission pool: %d", info.NodeEmissionPoolCNX)
	}
}

func TestGetCurrentEmissionWeekInfoReturnsCurrentIncompleteWeek(t *testing.T) {
	start := "2026-06-17T00:00:00Z"
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

	info, err := GetCurrentEmissionWeekInfo(now, start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if info.WeekIndex != 10 || info.YearIndex != 1 {
		t.Fatalf("unexpected week/year: week=%d year=%d", info.WeekIndex, info.YearIndex)
	}
	expectedStart := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	expectedEnd := expectedStart.Add(7 * 24 * time.Hour)
	if !info.WeekStartDate.Equal(expectedStart) || !info.WeekEndDate.Equal(expectedEnd) {
		t.Fatalf("unexpected week window: [%s, %s)", info.WeekStartDate, info.WeekEndDate)
	}
	if info.NodeEmissionPoolCNX != 1350649 {
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
	if info.NodeEmissionPoolCNX != 2182388 {
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
	mainnetStart := "2026-01-01T00:00:00Z"
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
	aligned, err := ParseMainnetAlignedWeekStart("2026-01-01T09:33:00Z")
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
	db := newEmissionSupplyTestDB(t)
	supply, err := GetCNXCirculatingSupply(
		context.Background(),
		db,
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
	db := newEmissionSupplyTestDB(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	supply, err := GetCNXCirculatingSupply(context.Background(), db, now, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	expected := supplyPercent(GetCNXTotalSupply(), year0TreasuryAllocationPercent)
	if supply.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, supply)
	}
}

func TestGetCNXCirculatingSupplyIncludesTransitionReleaseOnce(t *testing.T) {
	db := newEmissionSupplyTestDB(t)
	mainnetStart := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	released := cnxToWei(year1NodeReleasedBeforeTransitionCNX)
	records := []models.VestingRecord{
		{
			Address:        "0xdeprecated",
			TotalAmount:    models.BigInt{Int: *cnxToWei(20000000)},
			ReleasedAmount: models.BigInt{Int: *released},
			StartTime:      mainnetStart,
			DurationDays:   180,
			Type:           models.VestingTypeNode,
			AdminSignature: "0xsig",
			Status:         models.VestingStatusDeprecated,
		},
		{
			Address:        "0xactive",
			TotalAmount:    models.BigInt{Int: *cnxToWei(1350649)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      now,
			DurationDays:   180,
			Type:           models.VestingTypeNode,
			AdminSignature: "0xsig",
			Status:         models.VestingStatusActive,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create vesting records: %v", err)
	}

	supply, err := GetCNXCirculatingSupply(context.Background(), db, now, "2026-06-17T00:00:00Z")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	year0Unlocked := supplyPercent(GetCNXTotalSupply(), year0TreasuryAllocationPercent)

	expected := big.NewInt(0)
	expected.Add(expected, year0Unlocked)
	expected.Add(expected, released)
	expected.Add(expected, cnxToWei(10*(weeklyEmissionCNXByYear[0]*bootstrapTreasuryAllocationPercent/100)))

	if supply.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, supply)
	}
}

func TestYear1TransitionNodeBootstrapSchedule(t *testing.T) {
	expectedTransitionTime := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if !year1TransitionTime().Equal(expectedTransitionTime) {
		t.Fatalf("unexpected transition time: %s", year1TransitionTime())
	}
	if got := nodeBootstrapEmissionCNX(year1TransitionFirstEmissionWeekIndex); got != 1350649 {
		t.Fatalf("unexpected first transition emission: %d", got)
	}
	if got := nodeBootstrapEmissionCNX(year1TransitionFirstEmissionWeekIndex + 1); got != 1350649 {
		t.Fatalf("unexpected regular transition emission: %d", got)
	}
	if got := nodeBootstrapEmissionCNX(emissionWeeksPerYear - 1); got != 1350650 {
		t.Fatalf("unexpected final transition emission: %d", got)
	}

	total := int64(0)
	for weekIndex := year1TransitionFirstEmissionWeekIndex; weekIndex < emissionWeeksPerYear; weekIndex++ {
		total += nodeBootstrapEmissionCNX(weekIndex)
	}
	expected := int64(year1NodeBootstrapTargetCNX - year1NodeReleasedBeforeTransitionCNX)
	if total != expected {
		t.Fatalf("expected transition total %d, got %d", expected, total)
	}
}

func TestYear2NodeBootstrapScheduleUsesEightyPercent(t *testing.T) {
	if got := nodeBootstrapEmissionCNX(emissionWeeksPerYear); got != 2182388 {
		t.Fatalf("unexpected Year 2 node bootstrap emission: %d", got)
	}
}

func TestBootstrapScheduleAppliesRoundingRemainderToFinalWeek(t *testing.T) {
	finalWeekIndex := maxEmissionYear*emissionWeeksPerYear - 1
	if got := weeklyBootstrapEmissionCNX(finalWeekIndex); got != 270221 {
		t.Fatalf("unexpected final bootstrap emission: %d", got)
	}
	if got := nodeBootstrapEmissionCNX(finalWeekIndex); got != 216176 {
		t.Fatalf("unexpected final node bootstrap emission: %d", got)
	}

	total := int64(0)
	for weekIndex := 0; weekIndex <= finalWeekIndex; weekIndex++ {
		total += weeklyBootstrapEmissionCNX(weekIndex)
	}
	if total != 1723466652 {
		t.Fatalf("unexpected bootstrap schedule total: %d", total)
	}
}
