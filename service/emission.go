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
	emissionWeekDuration = 7 * 24 * time.Hour
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

	completedWeeks := int(now.UTC().Sub(startDate) / emissionWeekDuration)
	if completedWeeks <= 0 {
		return nil, ErrNoCompletedEmissionWeek
	}

	weekIndex := completedWeeks - 1
	yearIndex := weekIndex/emissionWeeksPerYear + 1
	if yearIndex < 1 || yearIndex > maxEmissionYear {
		return nil, fmt.Errorf("%w: year=%d week_index=%d", ErrEmissionWeekOutOfRange, yearIndex, weekIndex)
	}

	weekStart := startDate.Add(time.Duration(weekIndex) * emissionWeekDuration)
	weekEnd := weekStart.Add(emissionWeekDuration)
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
	startUTC := t.UTC()
	return time.Date(startUTC.Year(), startUTC.Month(), startUTC.Day(), 0, 0, 0, 0, time.UTC), nil
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

	nowUTC := now.UTC()
	elapsedWeeks := int(nowUTC.Sub(mainnetWeekStart) / emissionWeekDuration)
	if elapsedWeeks < 0 {
		elapsedWeeks = 0
	}

	currentWeekStart := mainnetWeekStart.Add(time.Duration(elapsedWeeks) * emissionWeekDuration)
	rangeStart := currentWeekStart.Add(-time.Duration(weeks) * emissionWeekDuration)
	weekStarts := make([]time.Time, 0, weeks)
	for i := 0; i < weeks; i++ {
		weekStarts = append(weekStarts, rangeStart.Add(time.Duration(i)*emissionWeekDuration))
	}

	return &EmissionChartRange{
		MainnetWeekStart: mainnetWeekStart,
		RangeStart:       rangeStart,
		RangeEnd:         currentWeekStart,
		WeekStarts:       weekStarts,
	}, nil
}

func AlignToMainnetEmissionWeekStart(vestingStartTime, mainnetWeekStart time.Time) (time.Time, bool) {
	vestingStartUTC := vestingStartTime.UTC()
	if vestingStartUTC.Before(mainnetWeekStart) {
		return time.Time{}, false
	}

	weekIndex := int(vestingStartUTC.Sub(mainnetWeekStart) / emissionWeekDuration)
	return mainnetWeekStart.Add(time.Duration(weekIndex) * emissionWeekDuration), true
}

func parseMainnetStartDate(raw string) (time.Time, error) {
	return ParseMainnetAlignedWeekStart(raw)
}
