package relayaccount

import (
	"context"
	"crynux_relay/models"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVestingRecordsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.VestingRecord{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createVestingRecord(t *testing.T, db *gorm.DB, rec models.VestingRecord, createdAt time.Time) models.VestingRecord {
	t.Helper()
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("failed to create vesting record: %v", err)
	}
	if err := db.Model(&models.VestingRecord{}).
		Where("id = ?", rec.ID).
		Update("created_at", createdAt).Error; err != nil {
		t.Fatalf("failed to update created_at: %v", err)
	}
	if err := db.First(&rec, rec.ID).Error; err != nil {
		t.Fatalf("failed to reload vesting record: %v", err)
	}
	return rec
}

func TestQueryAddressVestingRecordsSortsByCreatedAtDescAndIDDesc(t *testing.T) {
	db := newVestingRecordsTestDB(t)
	address := "0xabc"
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	base := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(200)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		Source:         "airdrop",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}

	rec1 := base
	rec1.ExternalID = "item-1"
	rec1 = createVestingRecord(t, db, rec1, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	rec2 := base
	rec2.ExternalID = "item-2"
	rec2 = createVestingRecord(t, db, rec2, time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))

	rec3 := base
	rec3.ExternalID = "item-3"
	rec3 = createVestingRecord(t, db, rec3, time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))

	records, total, err := queryAddressVestingRecords(context.Background(), db, address, 1, 20, now)
	if err != nil {
		t.Fatalf("query vesting records failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].ID != rec3.ID || records[1].ID != rec2.ID || records[2].ID != rec1.ID {
		t.Fatalf("unexpected order: [%d, %d, %d]", records[0].ID, records[1].ID, records[2].ID)
	}
	if records[0].Type != models.VestingTypeNode {
		t.Fatalf("expected type %s, got %s", models.VestingTypeNode, records[0].Type)
	}
}

func TestQueryAddressVestingRecordsComputesAmountsAndPaginates(t *testing.T) {
	db := newVestingRecordsTestDB(t)
	address := "0xabc"
	now := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)

	recordA := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(100)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		Source:         "airdrop",
		ExternalID:     "item-a",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}
	recordB := models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(1100)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           models.VestingTypeDelegation,
		Source:         "airdrop",
		ExternalID:     "item-b",
		AdminSignature: "0xsig",
		Status:         models.VestingStatusCompleted,
	}
	_ = createVestingRecord(t, db, recordA, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	_ = createVestingRecord(t, db, recordB, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	records, total, err := queryAddressVestingRecords(context.Background(), db, address, 1, 1, now)
	if err != nil {
		t.Fatalf("query vesting records failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record in page, got %d", len(records))
	}
	if records[0].RemainingAmount != "900" {
		t.Fatalf("expected remaining amount 900, got %s", records[0].RemainingAmount)
	}
	if records[0].LockedAmount != "500" {
		t.Fatalf("expected locked amount 500, got %s", records[0].LockedAmount)
	}
	if records[0].Type != models.VestingTypeNode {
		t.Fatalf("expected type %s, got %s", models.VestingTypeNode, records[0].Type)
	}

	records, _, err = queryAddressVestingRecords(context.Background(), db, address, 2, 1, now)
	if err != nil {
		t.Fatalf("query vesting records page 2 failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record in page 2, got %d", len(records))
	}
	if records[0].RemainingAmount != "0" {
		t.Fatalf("expected clamped remaining amount 0, got %s", records[0].RemainingAmount)
	}
	if records[0].Type != models.VestingTypeDelegation {
		t.Fatalf("expected type %s, got %s", models.VestingTypeDelegation, records[0].Type)
	}
}
