package models

import (
	"context"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListVestingDelegationEmissionDetailsByUserNodeNetworkAndStartTimeRangeScopesNetwork(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&VestingDelegationEmissionDetail{}); err != nil {
		t.Fatalf("failed to migrate vesting delegation emission detail: %v", err)
	}

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	details := []VestingDelegationEmissionDetail{
		{
			VestingRecordID: 1,
			UserAddress:     "0xuser",
			NodeAddress:     "0xnode",
			Network:         "base",
			TaskFee:         BigInt{Int: *big.NewInt(10)},
			EmissionAmount:  BigInt{Int: *big.NewInt(100)},
			StartTime:       start,
		},
		{
			VestingRecordID: 2,
			UserAddress:     "0xuser",
			NodeAddress:     "0xnode",
			Network:         "near",
			TaskFee:         BigInt{Int: *big.NewInt(20)},
			EmissionAmount:  BigInt{Int: *big.NewInt(200)},
			StartTime:       start,
		},
		{
			VestingRecordID: 3,
			UserAddress:     "0xuser",
			NodeAddress:     "0xnode",
			Network:         "base",
			TaskFee:         BigInt{Int: *big.NewInt(30)},
			EmissionAmount:  BigInt{Int: *big.NewInt(300)},
			StartTime:       start.Add(14 * 24 * time.Hour),
		},
	}
	if err := db.Create(&details).Error; err != nil {
		t.Fatalf("failed to create details: %v", err)
	}

	got, err := ListVestingDelegationEmissionDetailsByUserNodeNetworkAndStartTimeRange(
		context.Background(),
		db,
		"0xuser",
		"0xnode",
		"base",
		start,
		start.Add(7*24*time.Hour),
	)
	if err != nil {
		t.Fatalf("list details failed: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 base detail in range, got %d", len(got))
	}
	if got[0].Network != "base" || got[0].EmissionAmount.Int.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("unexpected detail: %+v", got[0])
	}
}
