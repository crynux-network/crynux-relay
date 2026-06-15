package service

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"crynux_relay/utils"
)

const (
	emissionWeeksPerYear = 52
	maxEmissionYear      = 20
	defaultChartWeeks    = 24
	maxChartWeeks        = 260
	emissionWeekDuration = 7 * 24 * time.Hour

	cnxTotalSupplyCNX          = 8617333262
	year0EmissionCNX           = 1723466646
	year0NodeAllocationPercent = 70
	year0VestingDurationDays   = 365
	nodeVestingDurationDays    = 180
	vestingDayDuration         = 24 * time.Hour
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

	nodeEmissionPool := weeklyEmission * nodeAllocationPercent(yearIndex) / 100

	return &EmissionWeekInfo{
		WeekIndex:           weekIndex,
		YearIndex:           yearIndex,
		WeekStartDate:       weekStart,
		WeekEndDate:         weekEnd,
		WeeklyEmissionCNX:   weeklyEmission,
		NodeEmissionPoolCNX: nodeEmissionPool,
	}, nil
}

func GetCNXTotalSupply() *big.Int {
	return cnxToWei(cnxTotalSupplyCNX)
}

func GetCNXCirculatingSupply(now time.Time, mainnetStartTime string) (*big.Int, error) {
	startDate, err := parseMainnetStartDate(mainnetStartTime)
	if err != nil {
		return nil, err
	}

	nowUTC := now.UTC()
	if nowUTC.Before(startDate) {
		return big.NewInt(0), nil
	}

	circulating := big.NewInt(0)

	year0Emission := cnxToWei(year0EmissionCNX)
	year0NodeVesting := cnxToWei(year0EmissionCNX * year0NodeAllocationPercent / 100)
	year0Unlocked := big.NewInt(0).Sub(year0Emission, year0NodeVesting)
	circulating.Add(circulating, year0Unlocked)
	circulating.Add(circulating, releasedVestingAmount(year0NodeVesting, startDate, year0VestingDurationDays, nowUTC))

	completedWeeks := int(nowUTC.Sub(startDate) / emissionWeekDuration)
	maxWeeks := maxEmissionYear * emissionWeeksPerYear
	if completedWeeks > maxWeeks {
		completedWeeks = maxWeeks
	}

	for weekIndex := 0; weekIndex < completedWeeks; weekIndex++ {
		yearIndex := weekIndex/emissionWeeksPerYear + 1
		weeklyEmissionCNX := weeklyEmissionCNXByYear[yearIndex-1]
		weeklyEmission := cnxToWei(weeklyEmissionCNX)
		nodeVesting := cnxToWei(weeklyEmissionCNX * nodeAllocationPercent(yearIndex) / 100)

		circulating.Add(circulating, big.NewInt(0).Sub(weeklyEmission, nodeVesting))

		vestingStart := startDate.Add(time.Duration(weekIndex+1) * emissionWeekDuration)
		circulating.Add(circulating, releasedVestingAmount(nodeVesting, vestingStart, nodeVestingDurationDays, nowUTC))
	}

	totalSupply := GetCNXTotalSupply()
	if circulating.Cmp(totalSupply) > 0 {
		return totalSupply, nil
	}
	return circulating, nil
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

func nodeAllocationPercent(yearIndex int) int64 {
	if yearIndex == 1 {
		return 70
	}
	return 80
}

func cnxToWei(amount int64) *big.Int {
	return utils.EtherToWei(big.NewInt(amount))
}

func releasedVestingAmount(totalAmount *big.Int, startTime time.Time, durationDays int, now time.Time) *big.Int {
	if durationDays <= 0 || !now.After(startTime) {
		return big.NewInt(0)
	}

	elapsedDays := int(now.Sub(startTime) / vestingDayDuration)
	if elapsedDays <= 0 {
		return big.NewInt(0)
	}
	if elapsedDays >= durationDays {
		return big.NewInt(0).Set(totalAmount)
	}

	released := big.NewInt(0).Mul(totalAmount, big.NewInt(int64(elapsedDays)))
	return released.Div(released, big.NewInt(int64(durationDays)))
}
