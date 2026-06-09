package service

import (
	"errors"
	"fmt"
	"time"
)

const (
	emissionWeeksPerYear = 52
	maxEmissionYear      = 20
	defaultChartWeeks    = 24
	maxChartWeeks        = 260
)

var (
	ErrMainnetStartTimeNotSet  = errors.New("dao.mainnet_start_time is not set")
	ErrInvalidMainnetStart     = errors.New("dao.mainnet_start_time must be RFC3339 format")
	ErrNoCompletedEmissionWeek = errors.New("no completed emission week yet")
	ErrEmissionWeekOutOfRange  = errors.New("emission week is out of Year 1-20 range")
	ErrInvalidChartWeeks       = errors.New("weeks must be between 1 and 260")
)

// weeklyEmissionCNXByYear stores tokenomics weekly emissions for Year 1 to Year 20.
var weeklyEmissionCNXByYear = []int64{
	13257447, 12171771, 11175003, 10259862, 9419664,
	8648271, 7940049, 7289824, 6692848, 6144758,
	5641553, 5179556, 4755393, 4365966, 4008429,
	3680172, 3378796, 3102100, 2848064, 2614832,
}

type EmissionWeekInfo struct {
	WeekIndex           int
	YearIndex           int
	WeekStartDate       time.Time
	WeekEndDate         time.Time
	WeeklyEmissionCNX   int64
	NodeEmissionPoolCNX int64
}

type EmissionChartRange struct {
	MainnetWeekStart time.Time
	RangeStart       time.Time
	RangeEnd         time.Time
	WeekStarts       []time.Time
}

func GetPreviousEmissionWeekInfo(now time.Time, mainnetStartTime string) (*EmissionWeekInfo, error) {
	startDate, err := parseMainnetStartDate(mainnetStartTime)
	if err != nil {
		return nil, err
	}

	today := now.UTC().Truncate(24 * time.Hour)
	elapsedDays := int(today.Sub(startDate) / (24 * time.Hour))
	completedWeeks := elapsedDays / 7
	if completedWeeks <= 0 {
		return nil, ErrNoCompletedEmissionWeek
	}

	weekIndex := completedWeeks - 1
	yearIndex := weekIndex/emissionWeeksPerYear + 1
	if yearIndex < 1 || yearIndex > maxEmissionYear {
		return nil, fmt.Errorf("%w: year=%d week_index=%d", ErrEmissionWeekOutOfRange, yearIndex, weekIndex)
	}

	weekStart := startDate.AddDate(0, 0, weekIndex*7)
	weekEnd := weekStart.AddDate(0, 0, 7)
	weeklyEmission := weeklyEmissionCNXByYear[yearIndex-1]

	nodeAllocationPercent := int64(80)
	if yearIndex == 1 {
		nodeAllocationPercent = 70
	}

	nodeEmissionPool := weeklyEmission * nodeAllocationPercent / 100

	return &EmissionWeekInfo{
		WeekIndex:           weekIndex,
		YearIndex:           yearIndex,
		WeekStartDate:       weekStart,
		WeekEndDate:         weekEnd,
		WeeklyEmissionCNX:   weeklyEmission,
		NodeEmissionPoolCNX: nodeEmissionPool,
	}, nil
}

func NormalizeToUTCWeekStart(t time.Time) time.Time {
	utcDay := t.UTC().Truncate(24 * time.Hour)
	offset := (int(utcDay.Weekday()) + 6) % 7
	return utcDay.AddDate(0, 0, -offset)
}

func ParseMainnetAlignedWeekStart(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, ErrMainnetStartTimeNotSet
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidMainnetStart, err)
	}
	return NormalizeToUTCWeekStart(t), nil
}

func ClampChartWeeks(weeks *int) (int, error) {
	value := defaultChartWeeks
	if weeks != nil {
		value = *weeks
	}
	if value <= 0 || value > maxChartWeeks {
		return 0, ErrInvalidChartWeeks
	}
	return value, nil
}

func BuildEmissionChartRange(now time.Time, mainnetStartTime string, weeks int) (*EmissionChartRange, error) {
	if weeks <= 0 || weeks > maxChartWeeks {
		return nil, ErrInvalidChartWeeks
	}

	mainnetWeekStart, err := ParseMainnetAlignedWeekStart(mainnetStartTime)
	if err != nil {
		return nil, err
	}

	today := now.UTC().Truncate(24 * time.Hour)
	elapsedDays := int(today.Sub(mainnetWeekStart) / (24 * time.Hour))
	if elapsedDays <= 0 {
		return &EmissionChartRange{
			MainnetWeekStart: mainnetWeekStart,
			RangeStart:       mainnetWeekStart,
			RangeEnd:         mainnetWeekStart,
			WeekStarts:       []time.Time{},
		}, nil
	}

	completedWeeks := elapsedDays / 7
	currentWeekStart := mainnetWeekStart.AddDate(0, 0, completedWeeks*7)
	if completedWeeks <= 0 {
		return &EmissionChartRange{
			MainnetWeekStart: mainnetWeekStart,
			RangeStart:       currentWeekStart,
			RangeEnd:         currentWeekStart,
			WeekStarts:       []time.Time{},
		}, nil
	}

	if weeks > completedWeeks {
		weeks = completedWeeks
	}
	rangeStart := currentWeekStart.AddDate(0, 0, -weeks*7)
	weekStarts := make([]time.Time, 0, weeks)
	for i := 0; i < weeks; i++ {
		weekStarts = append(weekStarts, rangeStart.AddDate(0, 0, i*7))
	}

	return &EmissionChartRange{
		MainnetWeekStart: mainnetWeekStart,
		RangeStart:       rangeStart,
		RangeEnd:         currentWeekStart,
		WeekStarts:       weekStarts,
	}, nil
}

func AlignToMainnetEmissionWeekStart(vestingStartTime, mainnetWeekStart time.Time) (time.Time, bool) {
	normalizedVestingStart := NormalizeToUTCWeekStart(vestingStartTime)
	if normalizedVestingStart.Before(mainnetWeekStart) {
		return time.Time{}, false
	}

	elapsedDays := int(normalizedVestingStart.Sub(mainnetWeekStart) / (24 * time.Hour))
	weekIndex := elapsedDays / 7
	return mainnetWeekStart.AddDate(0, 0, weekIndex*7), true
}

func parseMainnetStartDate(raw string) (time.Time, error) {
	return ParseMainnetAlignedWeekStart(raw)
}
