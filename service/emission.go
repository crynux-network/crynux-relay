package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"fmt"
	"math/big"
	"time"

	"crynux_relay/utils"
	"gorm.io/gorm"
)

const (
	emissionWeeksPerYear = 52
	maxEmissionYear      = 20
	defaultChartWeeks    = 24
	maxChartWeeks        = 260
	emissionWeekDuration = 7 * 24 * time.Hour

	cnxTotalSupplyCNX                     = 8617333262
	year0TreasuryAllocationPercent        = 6
	bootstrapNodeAllocationPercent        = 80
	bootstrapTreasuryAllocationPercent    = 20
	year1NodeBootstrapTargetCNX           = 74318067
	year1NodeReleasedBeforeTransitionCNX  = 16240159
	year1TransitionFirstEmissionWeekIndex = 9
	year1TransitionEmissionCount          = emissionWeeksPerYear - year1TransitionFirstEmissionWeekIndex
	year1TransitionTimeRFC3339            = "2026-08-26T00:00:00Z"
	bootstrapFinalWeekRemainderCNX        = 24
)

var (
	ErrMainnetStartTimeNotSet  = errors.New("dao.mainnet_start_time is not set")
	ErrInvalidMainnetStart     = errors.New("dao.mainnet_start_time must be RFC3339 format")
	ErrNoCompletedEmissionWeek = errors.New("no completed emission week yet")
	ErrEmissionWeekOutOfRange  = errors.New("emission week is out of Year 1-20 range")
	ErrInvalidChartWeeks       = errors.New("weeks must be between 1 and 260")
)

// weeklyEmissionCNXByYear stores bootstrap mining weekly emissions for Year 1 to Year 20.
var weeklyEmissionCNXByYear = []int64{
	1786492, 2727986, 3042483, 3101410, 3010312,
	2827694, 2592055, 2330245, 2061165, 1797889,
	1549089, 1320097, 1113750, 931051, 771710,
	634560, 517890, 419687, 337827, 270197,
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
	weeklyEmission := weeklyBootstrapEmissionCNX(weekIndex)

	nodeEmissionPool := nodeBootstrapEmissionCNX(weekIndex)

	return &EmissionWeekInfo{
		WeekIndex:           weekIndex,
		YearIndex:           yearIndex,
		WeekStartDate:       weekStart,
		WeekEndDate:         weekEnd,
		WeeklyEmissionCNX:   weeklyEmission,
		NodeEmissionPoolCNX: nodeEmissionPool,
	}, nil
}

func GetCurrentEmissionWeekInfo(now time.Time, mainnetStartTime string) (*EmissionWeekInfo, error) {
	startDate, err := parseMainnetStartDate(mainnetStartTime)
	if err != nil {
		return nil, err
	}

	elapsedWeeks := int(now.UTC().Sub(startDate) / emissionWeekDuration)
	if elapsedWeeks < 0 {
		elapsedWeeks = 0
	}

	yearIndex := elapsedWeeks/emissionWeeksPerYear + 1
	if yearIndex < 1 || yearIndex > maxEmissionYear {
		return nil, fmt.Errorf("%w: year=%d week_index=%d", ErrEmissionWeekOutOfRange, yearIndex, elapsedWeeks)
	}

	weekStart := startDate.Add(time.Duration(elapsedWeeks) * emissionWeekDuration)
	weekEnd := weekStart.Add(emissionWeekDuration)
	weeklyEmission := weeklyBootstrapEmissionCNX(elapsedWeeks)
	nodeEmissionPool := nodeBootstrapEmissionCNX(elapsedWeeks)

	return &EmissionWeekInfo{
		WeekIndex:           elapsedWeeks,
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

func GetCNXCirculatingSupply(ctx context.Context, db *gorm.DB, now time.Time, mainnetStartTime string) (*big.Int, error) {
	startDate, err := parseMainnetStartDate(mainnetStartTime)
	if err != nil {
		return nil, err
	}

	nowUTC := now.UTC()
	if nowUTC.Before(startDate) {
		return big.NewInt(0), nil
	}

	circulating := big.NewInt(0)

	totalSupply := GetCNXTotalSupply()
	year0Unlocked := supplyPercent(totalSupply, year0TreasuryAllocationPercent)
	circulating.Add(circulating, year0Unlocked)

	completedWeeks := int(nowUTC.Sub(startDate) / emissionWeekDuration)
	maxWeeks := maxEmissionYear * emissionWeeksPerYear
	if completedWeeks > maxWeeks {
		completedWeeks = maxWeeks
	}

	var vestingRecords []models.VestingRecord
	if err := db.WithContext(ctx).
		Model(&models.VestingRecord{}).
		Select("released_amount").
		Find(&vestingRecords).Error; err != nil {
		return nil, err
	}
	for _, record := range vestingRecords {
		circulating.Add(circulating, &record.ReleasedAmount.Int)
	}
	for weekIndex := 0; weekIndex < completedWeeks; weekIndex++ {
		weeklyEmissionCNX := weeklyBootstrapEmissionCNX(weekIndex)
		treasuryEmissionCNX := weeklyEmissionCNX * bootstrapTreasuryAllocationPercent / 100
		circulating.Add(circulating, cnxToWei(treasuryEmissionCNX))
	}

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
	rangeStart := currentWeekStart.Add(-time.Duration(weeks-1) * emissionWeekDuration)
	rangeEnd := currentWeekStart.Add(emissionWeekDuration)
	weekStarts := make([]time.Time, 0, weeks)
	for i := 0; i < weeks; i++ {
		weekStarts = append(weekStarts, rangeStart.Add(time.Duration(i)*emissionWeekDuration))
	}

	return &EmissionChartRange{
		MainnetWeekStart: mainnetWeekStart,
		RangeStart:       rangeStart,
		RangeEnd:         rangeEnd,
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

func nodeBootstrapEmissionCNX(weekIndex int) int64 {
	yearIndex := weekIndex/emissionWeeksPerYear + 1
	if yearIndex < 1 || yearIndex > maxEmissionYear {
		return 0
	}
	if yearIndex == 1 {
		if weekIndex < year1TransitionFirstEmissionWeekIndex {
			return 0
		}
		remaining := int64(year1NodeBootstrapTargetCNX - year1NodeReleasedBeforeTransitionCNX)
		emissionCount := int64(year1TransitionEmissionCount)
		base := remaining / emissionCount
		if weekIndex == emissionWeeksPerYear-1 {
			return base + remaining%emissionCount
		}
		return base
	}
	return weeklyBootstrapEmissionCNX(weekIndex) * bootstrapNodeAllocationPercent / 100
}

func weeklyBootstrapEmissionCNX(weekIndex int) int64 {
	yearIndex := weekIndex/emissionWeeksPerYear + 1
	if yearIndex < 1 || yearIndex > maxEmissionYear {
		return 0
	}
	weeklyEmission := weeklyEmissionCNXByYear[yearIndex-1]
	if weekIndex == maxEmissionYear*emissionWeeksPerYear-1 {
		weeklyEmission += bootstrapFinalWeekRemainderCNX
	}
	return weeklyEmission
}

func year1TransitionTime() time.Time {
	transitionTime, _ := time.Parse(time.RFC3339, year1TransitionTimeRFC3339)
	return transitionTime
}

func supplyPercent(totalSupply *big.Int, percent int64) *big.Int {
	amount := big.NewInt(0).Mul(totalSupply, big.NewInt(percent))
	return amount.Div(amount, big.NewInt(100))
}

func cnxToWei(amount int64) *big.Int {
	return utils.EtherToWei(big.NewInt(amount))
}
