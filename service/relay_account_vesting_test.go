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

func newRelayAccountVestingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.VestingRecord{}, &models.RelayAccountEvent{}, &models.RelayAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestNormalizeVestingInputRejectsAmountAboveUint256(t *testing.T) {
	tooLargeAmount := big.NewInt(1)
	tooLargeAmount.Lsh(tooLargeAmount, 256)

	_, _, err := normalizeVestingInput(CreateVestingRecordInput{
		Address:      "0x0000000000000000000000000000000000000001",
		TotalAmount:  tooLargeAmount.String(),
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		DurationDays: 10,
		Source:       "airdrop",
		ExternalID:   "item-1",
	})
	if err != ErrInvalidVestingAmount {
		t.Fatalf("expected ErrInvalidVestingAmount, got %v", err)
	}
}

func TestProcessDueVestingReleasesSkipsWhenPendingReleaseExists(t *testing.T) {
	ctx := context.Background()
	db := newRelayAccountVestingTestDB(t)
	address := "0xabc"
	record := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(200)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Source:         "airdrop",
		ExternalID:     "item-1",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("failed to create vesting record: %v", err)
	}
	pendingEvent := models.RelayAccountEvent{
		Address: address,
		Amount:  models.BigInt{Int: *big.NewInt(100)},
		Status:  models.RelayAccountEventStatusPending,
		Type:    models.RelayAccountEventTypeVestingRelease,
		Reason:  models.BuildVestingReleaseReason(record.ID, big.NewInt(0), big.NewInt(100)),
	}
	if err := db.Create(&pendingEvent).Error; err != nil {
		t.Fatalf("failed to create pending release event: %v", err)
	}

	err := ProcessDueVestingReleases(ctx, db, record.StartTime.Add(3*24*time.Hour), 100)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var updated models.VestingRecord
	if err := db.First(&updated, record.ID).Error; err != nil {
		t.Fatalf("failed to load vesting record: %v", err)
	}
	if updated.ReleasedAmount.String() != "200" {
		t.Fatalf("expected released amount to stay 200, got %s", updated.ReleasedAmount.String())
	}

	var releaseEventCount int64
	if err := db.Model(&models.RelayAccountEvent{}).
		Where("type = ?", models.RelayAccountEventTypeVestingRelease).
		Count(&releaseEventCount).Error; err != nil {
		t.Fatalf("failed to count release events: %v", err)
	}
	if releaseEventCount != 1 {
		t.Fatalf("expected no new release events, got %d", releaseEventCount)
	}
}

func TestReleaseVestingToRelayAccountRejectsDuplicateRange(t *testing.T) {
	ctx := context.Background()
	db := newRelayAccountVestingTestDB(t)
	address := "0xabc"

	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = make(map[string]*big.Int)
	relayAccountCache.mu.Unlock()

	commit, err := releaseVestingToRelayAccount(ctx, db, 42, address, big.NewInt(0), big.NewInt(100))
	if err != nil {
		t.Fatalf("expected first release to pass, got %v", err)
	}
	if err := commit(); err != nil {
		t.Fatalf("commit first release: %v", err)
	}

	_, err = releaseVestingToRelayAccount(ctx, db, 42, address, big.NewInt(0), big.NewInt(100))
	if !errors.Is(err, ErrVestingReleaseRangeInvalid) {
		t.Fatalf("expected ErrVestingReleaseRangeInvalid, got %v", err)
	}
}
