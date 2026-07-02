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
	if err := db.AutoMigrate(&models.Node{}, &models.VestingRecord{}, &models.RelayAccountEvent{}, &models.RelayAccount{}); err != nil {
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
		Type:         models.VestingTypeNode,
		Source:       "airdrop",
		ExternalID:   "item-1",
	})
	if err != ErrInvalidVestingAmount {
		t.Fatalf("expected ErrInvalidVestingAmount, got %v", err)
	}
}

func TestNormalizeVestingInputRejectsInvalidType(t *testing.T) {
	_, _, err := normalizeVestingInput(CreateVestingRecordInput{
		Address:      "0x0000000000000000000000000000000000000001",
		TotalAmount:  "1000",
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		DurationDays: 10,
		Type:         "invalid",
		Source:       "airdrop",
		ExternalID:   "item-1",
	})
	if err != ErrInvalidVestingType {
		t.Fatalf("expected ErrInvalidVestingType, got %v", err)
	}
}

func TestNormalizeVestingDelegationDetailsRequiresAggregateSum(t *testing.T) {
	payload := vestingSignPayload{
		Address:      "0x0000000000000000000000000000000000000001",
		TotalAmount:  "1000",
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		DurationDays: 10,
		Type:         models.VestingTypeDelegation,
		Source:       "emission",
		ExternalID:   "item-1",
	}
	input := CreateVestingRecordInput{
		DelegationDetails: []CreateVestingDelegationDetailInput{
			{
				UserAddress:      payload.Address,
				NodeAddress:      "0x0000000000000000000000000000000000000002",
				Network:          "base",
				TaskFee:          "500",
				EmissionAmount:   "400",
				Source:           "emission",
				DetailExternalID: "detail-1",
				StartTime:        payload.StartTime,
			},
		},
	}

	_, err := normalizeVestingDelegationDetails(input, payload, big.NewInt(1000))
	if !errors.Is(err, ErrInvalidVestingDelegationDetail) {
		t.Fatalf("expected ErrInvalidVestingDelegationDetail, got %v", err)
	}
}

func TestNormalizeVestingDelegationDetailsBuildsDetailRows(t *testing.T) {
	payload := vestingSignPayload{
		Address:      "0x0000000000000000000000000000000000000001",
		TotalAmount:  "1000",
		StartTime:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		DurationDays: 10,
		Type:         models.VestingTypeDelegation,
		Source:       "emission",
		ExternalID:   "item-1",
	}
	input := CreateVestingRecordInput{
		DelegationDetails: []CreateVestingDelegationDetailInput{
			{
				UserAddress:      payload.Address,
				NodeAddress:      "0x0000000000000000000000000000000000000002",
				Network:          "base",
				TaskFee:          "500",
				EmissionAmount:   "400",
				Source:           "emission",
				DetailExternalID: "detail-1",
				StartTime:        payload.StartTime,
			},
			{
				UserAddress:      payload.Address,
				NodeAddress:      "0x0000000000000000000000000000000000000003",
				Network:          "near",
				TaskFee:          "700",
				EmissionAmount:   "600",
				Source:           "emission",
				DetailExternalID: "detail-2",
				StartTime:        payload.StartTime,
			},
		},
	}

	details, err := normalizeVestingDelegationDetails(input, payload, big.NewInt(1000))
	if err != nil {
		t.Fatalf("expected valid details, got %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(details))
	}
	if details[0].EmissionAmount.String() != "400" || details[1].EmissionAmount.String() != "600" {
		t.Fatalf("unexpected details: %+v", details)
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
		Type:           models.VestingTypeNode,
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

func TestGetAddressLockedVestingAmountOnlyCountsActiveRecords(t *testing.T) {
	ctx := context.Background()
	db := newRelayAccountVestingTestDB(t)
	address := "0xabc"
	now := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	activeRecord := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		Source:         "airdrop",
		ExternalID:     "active-1",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}
	completedRecord := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(4000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		Source:         "airdrop",
		ExternalID:     "completed-1",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusCompleted,
	}
	if err := db.Create(&activeRecord).Error; err != nil {
		t.Fatalf("failed to create active record: %v", err)
	}
	if err := db.Create(&completedRecord).Error; err != nil {
		t.Fatalf("failed to create completed record: %v", err)
	}

	lockedAmount, err := GetAddressLockedVestingAmount(ctx, db, address, now)
	if err != nil {
		t.Fatalf("get locked amount failed: %v", err)
	}

	if lockedAmount.String() != "800" {
		t.Fatalf("expected locked amount 800 from active vesting only, got %s", lockedAmount.String())
	}
}

func TestGetAddressLockedVestingAmountExcludesSlashedRecords(t *testing.T) {
	ctx := context.Background()
	db := newRelayAccountVestingTestDB(t)
	address := "0xabc"
	now := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	records := []models.VestingRecord{
		{
			Address:        address,
			TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			DurationDays:   10,
			Type:           models.VestingTypeNode,
			Source:         "airdrop",
			ExternalID:     "active-1",
			AdminSignature: "0xsig",
			Status:         models.VestingStatusActive,
		},
		{
			Address:        address,
			TotalAmount:    models.BigInt{Int: *big.NewInt(4000)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			DurationDays:   10,
			Type:           models.VestingTypeNode,
			Source:         "airdrop",
			ExternalID:     "slashed-1",
			AdminSignature: "0xsig",
			Status:         models.VestingStatusActive,
			Slashed:        true,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("failed to create vesting records: %v", err)
	}

	lockedAmount, err := GetAddressLockedVestingAmount(ctx, db, address, now)
	if err != nil {
		t.Fatalf("get locked amount failed: %v", err)
	}
	if lockedAmount.String() != "800" {
		t.Fatalf("expected locked amount 800 from unslashed vesting only, got %s", lockedAmount.String())
	}
}

func TestProcessDueVestingReleasesSkipsSlashedRecords(t *testing.T) {
	ctx := context.Background()
	db := newRelayAccountVestingTestDB(t)
	record := models.VestingRecord{
		Address:        "0x0000000000000000000000000000000000000001",
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		Source:         "airdrop",
		ExternalID:     "slashed-1",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
		Slashed:        true,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("failed to create vesting record: %v", err)
	}

	if err := ProcessDueVestingReleases(ctx, db, record.StartTime.Add(3*24*time.Hour), 100); err != nil {
		t.Fatalf("release processing failed: %v", err)
	}

	var updated models.VestingRecord
	if err := db.First(&updated, record.ID).Error; err != nil {
		t.Fatalf("failed to load vesting record: %v", err)
	}
	if updated.ReleasedAmount.String() != "0" {
		t.Fatalf("expected released amount to stay 0, got %s", updated.ReleasedAmount.String())
	}

	var releaseEventCount int64
	if err := db.Model(&models.RelayAccountEvent{}).
		Where("type = ?", models.RelayAccountEventTypeVestingRelease).
		Count(&releaseEventCount).Error; err != nil {
		t.Fatalf("failed to count release events: %v", err)
	}
	if releaseEventCount != 0 {
		t.Fatalf("expected no release events, got %d", releaseEventCount)
	}
}

func TestRestoredSlashedVestingCatchesUpRelease(t *testing.T) {
	ctx := context.Background()
	db := newRelayAccountVestingTestDB(t)
	address := "0x0000000000000000000000000000000000000001"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	record := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(100)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeOther,
		Source:         "airdrop",
		ExternalID:     "restore-1",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
		Slashed:        true,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("failed to create vesting record: %v", err)
	}

	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{address: big.NewInt(0)}
	relayAccountCache.mu.Unlock()

	if err := RestoreNodeVestings(ctx, db, address, start.Add(5*24*time.Hour)); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if err := ProcessDueVestingReleases(ctx, db, start.Add(5*24*time.Hour), 100); err != nil {
		t.Fatalf("release processing failed: %v", err)
	}

	var updated models.VestingRecord
	if err := db.First(&updated, record.ID).Error; err != nil {
		t.Fatalf("failed to load vesting record: %v", err)
	}
	if updated.Slashed {
		t.Fatal("expected restored record to be unslashed")
	}
	if updated.ReleasedAmount.String() != "500" {
		t.Fatalf("expected released amount to catch up to 500, got %s", updated.ReleasedAmount.String())
	}
}
