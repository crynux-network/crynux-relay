package models

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type VestingStatus int8

const (
	VestingStatusActive VestingStatus = iota
	VestingStatusCompleted
)

const (
	VestingTypeNode       = "node"
	VestingTypeDelegation = "delegation"
	VestingTypeOther      = "other"
)

type VestingRecord struct {
	gorm.Model
	Address        string        `json:"address" gorm:"not null;index"`
	TotalAmount    BigInt        `json:"total_amount" gorm:"not null"`
	ReleasedAmount BigInt        `json:"released_amount" gorm:"not null"`
	StartTime      time.Time     `json:"start_time" gorm:"not null;index"`
	DurationDays   uint          `json:"duration_days" gorm:"not null"`
	Type           string        `json:"type" gorm:"not null;size:32;index"`
	Source         string        `json:"source" gorm:"not null;size:64;uniqueIndex:idx_vesting_source_external_id"`
	ExternalID     string        `json:"external_id" gorm:"not null;size:128;uniqueIndex:idx_vesting_source_external_id"`
	AdminSignature string        `json:"admin_signature" gorm:"not null;size:255"`
	Status         VestingStatus `json:"status" gorm:"not null;default:0;index"`
}

type VestingCreatedReasonPayload struct {
	VestingID      uint   `json:"vesting_id"`
	Address        string `json:"address"`
	TotalAmount    string `json:"total_amount"`
	ReleasedAmount string `json:"released_amount"`
	StartTime      int64  `json:"start_time"`
	DurationDays   uint   `json:"duration_days"`
	Type           string `json:"type"`
	Source         string `json:"source"`
	ExternalID     string `json:"external_id"`
	AdminSignature string `json:"admin_signature"`
}

func ComputeVestingShouldReleased(totalAmount *big.Int, startTime time.Time, durationDays uint, now time.Time) *big.Int {
	if totalAmount == nil || totalAmount.Sign() <= 0 || durationDays == 0 {
		return big.NewInt(0)
	}

	startUTC := startTime.UTC()
	nowUTC := now.UTC()
	if nowUTC.Before(startUTC) {
		return big.NewInt(0)
	}

	startDay := time.Date(startUTC.Year(), startUTC.Month(), startUTC.Day(), 0, 0, 0, 0, time.UTC)
	nowDay := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	elapsedDays := uint(nowDay.Sub(startDay) / (24 * time.Hour))
	if elapsedDays >= durationDays {
		return new(big.Int).Set(totalAmount)
	}

	shouldReleased := big.NewInt(0).Mul(totalAmount, big.NewInt(0).SetUint64(uint64(elapsedDays)))
	shouldReleased.Div(shouldReleased, big.NewInt(0).SetUint64(uint64(durationDays)))
	return shouldReleased
}

func (v *VestingRecord) LockedAmountAt(now time.Time) *big.Int {
	shouldReleased := ComputeVestingShouldReleased(&v.TotalAmount.Int, v.StartTime, v.DurationDays, now)
	locked := big.NewInt(0).Sub(&v.TotalAmount.Int, shouldReleased)
	if locked.Sign() < 0 {
		return big.NewInt(0)
	}
	return locked
}

func BuildVestingCreatedReason(record VestingRecord) string {
	return fmt.Sprintf("%d-%d", RelayAccountEventTypeVestingCreated, record.ID)
}

func BuildVestingCreatedPayload(record VestingRecord) VestingCreatedReasonPayload {
	return VestingCreatedReasonPayload{
		VestingID:      record.ID,
		Address:        record.Address,
		TotalAmount:    record.TotalAmount.String(),
		ReleasedAmount: "0",
		StartTime:      record.StartTime.Unix(),
		DurationDays:   record.DurationDays,
		Type:           record.Type,
		Source:         record.Source,
		ExternalID:     record.ExternalID,
		AdminSignature: record.AdminSignature,
	}
}

func BuildVestingReleaseReason(vestingID uint, fromReleased, toReleased *big.Int) string {
	return fmt.Sprintf("%d-%d-%s-%s", RelayAccountEventTypeVestingRelease, vestingID, fromReleased.String(), toReleased.String())
}

func ParseVestingCreatedReason(reason string) (uint, bool) {
	parts := strings.SplitN(reason, "-", 2)
	if len(parts) != 2 || parts[0] != strconv.Itoa(int(RelayAccountEventTypeVestingCreated)) {
		return 0, false
	}
	vestingID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	if vestingID == 0 {
		return 0, false
	}
	return uint(vestingID), true
}

func ParseVestingReleaseReason(reason string) (uint, *big.Int, *big.Int, bool) {
	parts := strings.SplitN(reason, "-", 4)
	if len(parts) != 4 || parts[0] != strconv.Itoa(int(RelayAccountEventTypeVestingRelease)) {
		return 0, nil, nil, false
	}
	vestingID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, nil, nil, false
	}
	fromReleased, ok := big.NewInt(0).SetString(parts[2], 10)
	if !ok {
		return 0, nil, nil, false
	}
	toReleased, ok := big.NewInt(0).SetString(parts[3], 10)
	if !ok {
		return 0, nil, nil, false
	}
	return uint(vestingID), fromReleased, toReleased, true
}

func ListVestingRecordsByAddressAndStartTimeRange(ctx context.Context, db *gorm.DB, address string, startTime, endTime time.Time) ([]VestingRecord, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var records []VestingRecord
	if err := db.WithContext(dbCtx).
		Model(&VestingRecord{}).
		Where("address = ?", address).
		Where("start_time >= ? AND start_time < ?", startTime, endTime).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func ListVestingRecordsByAddressAndTypeAndStartTimeRange(ctx context.Context, db *gorm.DB, address, vestingType string, startTime, endTime time.Time) ([]VestingRecord, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var records []VestingRecord
	if err := db.WithContext(dbCtx).
		Model(&VestingRecord{}).
		Where("address = ?", address).
		Where("type = ?", vestingType).
		Where("start_time >= ? AND start_time < ?", startTime, endTime).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func ListVestingRecordsByAddressAndTypesAndStartTimeRange(ctx context.Context, db *gorm.DB, address string, vestingTypes []string, startTime, endTime time.Time) ([]VestingRecord, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := db.WithContext(dbCtx).
		Model(&VestingRecord{}).
		Where("address = ?", address).
		Where("start_time >= ? AND start_time < ?", startTime, endTime)
	if len(vestingTypes) > 0 {
		query = query.Where("type IN (?)", vestingTypes)
	}

	var records []VestingRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
