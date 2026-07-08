package service

import (
	"context"
	"crynux_relay/models"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVestingStakeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Node{}, &models.VestingRecord{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func resetVestingStakeTestCaches(network string) {
	globalNodeVestingStakeCache = newNodeVestingStakeCache()
	globalDelegationCaches = map[string]*delegationCache{
		network: {
			nodeDelegations: make(map[string]map[string]*big.Int),
			userDelegations: make(map[string]map[string]*big.Int),
			userStakeAmount: make(map[string]*big.Int),
			nodeStakeAmount: make(map[string]*big.Int),
		},
	}
	globalMaxStaking = newMaxStaking()
}

func TestGetNodeScoreStakeAmountIncludesUnslashedLockedVestings(t *testing.T) {
	ctx := context.Background()
	db := newVestingStakeTestDB(t)
	network := "network-a"
	resetVestingStakeTestCaches(network)

	address := "0x0000000000000000000000000000000000000001"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(2 * 24 * time.Hour)
	node := models.Node{
		Address:     address,
		Network:     network,
		Status:      models.NodeStatusAvailable,
		StakeAmount: models.BigInt{Int: *big.NewInt(100)},
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}).Error; err != nil {
		t.Fatalf("failed to create active vesting: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(5000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start.Add(time.Hour),
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
		Slashed:        true,
	}).Error; err != nil {
		t.Fatalf("failed to create slashed vesting: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(7000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeDelegation,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}).Error; err != nil {
		t.Fatalf("failed to create delegation vesting: %v", err)
	}
	UpdateDelegation("0x0000000000000000000000000000000000000002", address, big.NewInt(30), network)

	if err := InitNodeVestingStakeCache(ctx, db); err != nil {
		t.Fatalf("failed to init node vesting stake cache: %v", err)
	}

	scoreStake := GetNodeScoreStakeAmount(node, now)
	if scoreStake.String() != "6530" {
		t.Fatalf("expected score stake 6530, got %s", scoreStake.String())
	}
}

func TestSlashAndRestoreNodeVestingsRefreshScoreStake(t *testing.T) {
	ctx := context.Background()
	db := newVestingStakeTestDB(t)
	network := "network-a"
	resetVestingStakeTestCaches(network)

	address := "0x0000000000000000000000000000000000000001"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(2 * 24 * time.Hour)
	node := models.Node{
		Address:     address,
		Network:     network,
		Status:      models.NodeStatusAvailable,
		StakeAmount: models.BigInt{Int: *big.NewInt(100)},
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}).Error; err != nil {
		t.Fatalf("failed to create vesting: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(2000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeOther,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}).Error; err != nil {
		t.Fatalf("failed to create other vesting: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(3000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeDelegation,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}).Error; err != nil {
		t.Fatalf("failed to create delegation vesting: %v", err)
	}

	if err := InitNodeVestingStakeCache(ctx, db); err != nil {
		t.Fatalf("failed to init node vesting stake cache: %v", err)
	}
	if err := SlashNodeVestings(ctx, db, address, now); err != nil {
		t.Fatalf("slash vesting failed: %v", err)
	}
	if got := GetNodeScoreStakeAmount(node, now); got.String() != "100" {
		t.Fatalf("expected score stake 100 after slash, got %s", got.String())
	}
	var slashedCount int64
	if err := db.Model(&models.VestingRecord{}).
		Where("address = ?", address).
		Where("slashed = ?", true).
		Count(&slashedCount).Error; err != nil {
		t.Fatalf("failed to count slashed vestings: %v", err)
	}
	if slashedCount != 3 {
		t.Fatalf("expected 3 slashed vesting records, got %d", slashedCount)
	}

	if err := RestoreNodeVestings(ctx, db, address, now); err != nil {
		t.Fatalf("restore vesting failed: %v", err)
	}
	if got := GetNodeScoreStakeAmount(node, now); got.String() != "4900" {
		t.Fatalf("expected score stake 4900 after restore, got %s", got.String())
	}
}

func TestRefreshNodeVestingScoreStakesClearsCompletedVestingCache(t *testing.T) {
	ctx := context.Background()
	db := newVestingStakeTestDB(t)
	network := "network-a"
	resetVestingStakeTestCaches(network)

	address := "0x0000000000000000000000000000000000000001"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(2 * 24 * time.Hour)
	node := models.Node{
		Address:     address,
		Network:     network,
		Status:      models.NodeStatusAvailable,
		StakeAmount: models.BigInt{Int: *big.NewInt(100)},
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := db.Create(&models.VestingRecord{
		Address:        address,
		TotalAmount:    models.BigInt{Int: *big.NewInt(1000)},
		ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
		StartTime:      start,
		DurationDays:   10,
		Type:           models.VestingTypeNode,
		AdminSignature: "0xsig",
		Status:         models.VestingStatusActive,
	}).Error; err != nil {
		t.Fatalf("failed to create vesting: %v", err)
	}

	if err := InitNodeVestingStakeCache(ctx, db); err != nil {
		t.Fatalf("failed to init node vesting stake cache: %v", err)
	}
	UpdateMaxStaking(address, GetNodeScoreStakeAmount(node, now))

	if err := db.Model(&models.VestingRecord{}).
		Where("address = ?", address).
		Updates(map[string]interface{}{
			"released_amount": models.BigInt{Int: *big.NewInt(1000)},
			"status":          models.VestingStatusCompleted,
		}).Error; err != nil {
		t.Fatalf("failed to complete vesting: %v", err)
	}
	if err := RefreshNodeVestingScoreStakes(ctx, db, now); err != nil {
		t.Fatalf("failed to refresh node vesting score stakes: %v", err)
	}

	if got := GetNodeScoreStakeAmount(node, now); got.String() != "100" {
		t.Fatalf("expected score stake 100 after completed vesting refresh, got %s", got.String())
	}
	if got := globalMaxStaking.stakingMap[address]; got == nil || got.String() != "100" {
		t.Fatalf("expected max-staking cache entry 100, got %v", got)
	}
}
