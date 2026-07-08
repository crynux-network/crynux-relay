package models

import (
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestComputeVestingShouldReleased(t *testing.T) {
	total := big.NewInt(0).SetUint64(1000)
	start := time.Date(2026, 1, 1, 13, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		now      time.Time
		duration uint
		expected string
	}{
		{
			name:     "before start",
			now:      start.Add(-time.Second),
			duration: 10,
			expected: "0",
		},
		{
			name:     "same day no release",
			now:      time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC),
			duration: 10,
			expected: "0",
		},
		{
			name:     "next day midnight releases first day",
			now:      time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			duration: 10,
			expected: "100",
		},
		{
			name:     "mid schedule",
			now:      time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			duration: 10,
			expected: "300",
		},
		{
			name:     "non UTC input uses UTC calendar day",
			now:      time.Date(2026, 1, 2, 7, 59, 59, 0, time.FixedZone("UTC+8", 8*60*60)),
			duration: 10,
			expected: "0",
		},
		{
			name:     "final day and beyond",
			now:      start.Add(15 * 24 * time.Hour),
			duration: 10,
			expected: "1000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeVestingShouldReleased(total, start, tc.duration, tc.now)
			if got.String() != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got.String())
			}
		})
	}
}

func TestVestingReasonRoundTrip(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	record := VestingRecord{
		Model:          gorm.Model{ID: 42},
		Address:        "0xabc",
		TotalAmount:    BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: BigInt{Int: *big.NewInt(500)},
		StartTime:      start,
		DurationDays:   10,
		Type:           VestingTypeNode,
		AdminSignature: "0xsig",
	}
	createdReason := BuildVestingCreatedReason(record)
	parsedCreatedID, ok := ParseVestingCreatedReason(createdReason)
	if !ok || parsedCreatedID != 42 {
		t.Fatalf("created reason parse failed: %s", createdReason)
	}
	createdPayload := BuildVestingCreatedPayload(record)
	if createdPayload.Address != record.Address ||
		createdPayload.TotalAmount != "1000" ||
		createdPayload.ReleasedAmount != "0" ||
		createdPayload.StartTime != start.Unix() ||
		createdPayload.DurationDays != record.DurationDays ||
		createdPayload.Type != record.Type ||
		createdPayload.AdminSignature != record.AdminSignature {
		t.Fatalf("created payload mismatch: %#v", createdPayload)
	}

	from := big.NewInt(125)
	to := big.NewInt(300)
	releaseReason := BuildVestingReleaseReason(42, from, to)
	parsedID, parsedFrom, parsedTo, ok := ParseVestingReleaseReason(releaseReason)
	if !ok {
		t.Fatalf("release reason parse failed: %s", releaseReason)
	}
	if parsedID != 42 || parsedFrom.Cmp(from) != 0 || parsedTo.Cmp(to) != 0 {
		t.Fatalf("release reason mismatch: id=%d from=%s to=%s", parsedID, parsedFrom.String(), parsedTo.String())
	}
}

func TestVestingRecordSlashedDefaultsFalse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&VestingRecord{}); err != nil {
		t.Fatalf("failed to migrate vesting records: %v", err)
	}

	record := VestingRecord{
		Address:        "0xabc",
		TotalAmount:    BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: BigInt{Int: *big.NewInt(0)},
		StartTime:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationDays:   10,
		Type:           VestingTypeNode,
		AdminSignature: "0xsig",
		Status:         VestingStatusActive,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("failed to create vesting record: %v", err)
	}

	var stored VestingRecord
	if err := db.First(&stored, record.ID).Error; err != nil {
		t.Fatalf("failed to load vesting record: %v", err)
	}
	if stored.Slashed {
		t.Fatal("expected slashed to default to false")
	}
}
