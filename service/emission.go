package service

import (
	"errors"
	"fmt"
	"time"
)

const (
	emissionWeeksPerYear = 52
	maxEmissionYear      = 20
)

var (
	ErrMainnetStartTimeNotSet  = errors.New("dao.mainnet_start_time is not set")
	ErrInvalidMainnetStart     = errors.New("dao.mainnet_start_time must be RFC3339 format")
	ErrNoCompletedEmissionWeek = errors.New("no completed emission week yet")
	ErrEmissionWeekOutOfRange  = errors.New("emission week is out of Year 1-20 range")
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

func parseMainnetStartDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, ErrMainnetStartTimeNotSet
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidMainnetStart, err)
	}
	return t.UTC().Truncate(24 * time.Hour), nil
}
