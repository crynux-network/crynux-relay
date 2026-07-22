package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"fmt"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDelegationTaskFeeLeaderboardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.UserStakingEarning{},
		&models.Delegation{},
		&models.DelegatedStakingNodeListSnapshot{},
		&models.DelegationTaskFeeLeaderboardSnapshot{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func createDelegationLeaderboardFixture(t *testing.T, db *gorm.DB, delegator, node string, dailyEarning, stakingAmount int64, day time.Time) {
	t.Helper()
	if err := db.Create(&models.UserStakingEarning{
		UserAddress: delegator,
		NodeAddress: node,
		Network:     "base",
		Earning:     models.BigInt{Int: *big.NewInt(dailyEarning)},
		Time:        sql.NullTime{Time: day, Valid: true},
	}).Error; err != nil {
		t.Fatalf("create user staking earning: %v", err)
	}
	if err := db.Create(&models.Delegation{
		DelegatorAddress: delegator,
		NodeAddress:      node,
		Network:          "base",
		Amount:           models.BigInt{Int: *big.NewInt(stakingAmount)},
	}).Error; err != nil {
		t.Fatalf("create delegation: %v", err)
	}
}

func TestRebuildDelegationTaskFeeLeaderboardEmptyData(t *testing.T) {
	db := setupDelegationTaskFeeLeaderboardTestDB(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

	if err := RebuildDelegationTaskFeeLeaderboardSnapshots(context.Background(), db, now); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	snapshots, err := GetDelegationTaskFeeLeaderboard(context.Background(), db)
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected empty leaderboard, got %d rows", len(snapshots))
	}
}

func TestRebuildDelegationTaskFeeLeaderboardSortsByTaskFeeAndJoinsStakingAndAPR(t *testing.T) {
	db := setupDelegationTaskFeeLeaderboardTestDB(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	day := now.Truncate(24 * time.Hour)

	createDelegationLeaderboardFixture(t, db, "0xdelegator-a", "0xnode-a", 100, 1000, day)
	createDelegationLeaderboardFixture(t, db, "0xdelegator-b", "0xnode-b", 300, 2000, day)
	createDelegationLeaderboardFixture(t, db, "0xdelegator-c", "0xnode-c", 200, 3000, day)
	if err := db.Create(&models.DelegatedStakingNodeListSnapshot{
		NodeAddress:            "0xnode-b",
		StatusGroup:            "running",
		GPUName:                "RTX 4090",
		DelegationApr12m:       0.42,
		DelegationAprUpdatedAt: day,
	}).Error; err != nil {
		t.Fatalf("create node list snapshot: %v", err)
	}

	if err := RebuildDelegationTaskFeeLeaderboardSnapshots(context.Background(), db, now); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	snapshots, err := GetDelegationTaskFeeLeaderboard(context.Background(), db)
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(snapshots))
	}
	expectedOrder := []string{"0xdelegator-b", "0xdelegator-c", "0xdelegator-a"}
	for i, delegator := range expectedOrder {
		if snapshots[i].DelegatorAddress != delegator {
			t.Fatalf("unexpected rank %d delegator %s", i+1, snapshots[i].DelegatorAddress)
		}
		if snapshots[i].Rank != uint8(i+1) {
			t.Fatalf("unexpected rank value %d at index %d", snapshots[i].Rank, i)
		}
	}
	if snapshots[0].StakingAmount.String() != "2000" {
		t.Fatalf("unexpected staking amount %s", snapshots[0].StakingAmount.String())
	}
	if snapshots[0].TaskFee.String() != "300" {
		t.Fatalf("unexpected task fee %s", snapshots[0].TaskFee.String())
	}
	if snapshots[0].Network != "base" {
		t.Fatalf("unexpected network %s", snapshots[0].Network)
	}
	if snapshots[0].GPUName != "RTX 4090" {
		t.Fatalf("unexpected GPU name %s", snapshots[0].GPUName)
	}
	if snapshots[0].DelegationApr12m != 0.42 {
		t.Fatalf("unexpected APR %f", snapshots[0].DelegationApr12m)
	}
	if snapshots[1].GPUName != "" {
		t.Fatalf("expected empty GPU name for node without snapshot, got %s", snapshots[1].GPUName)
	}
	if snapshots[1].DelegationApr12m != 0 {
		t.Fatalf("expected zero APR for node without snapshot, got %f", snapshots[1].DelegationApr12m)
	}
}

func TestRebuildDelegationTaskFeeLeaderboardLimitsToTopTen(t *testing.T) {
	db := setupDelegationTaskFeeLeaderboardTestDB(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	day := now.Truncate(24 * time.Hour)

	for i := 0; i < 12; i++ {
		createDelegationLeaderboardFixture(t, db, fmt.Sprintf("0xdelegator-%02d", i), fmt.Sprintf("0xnode-%02d", i), int64(100+i), 1000, day)
	}

	if err := RebuildDelegationTaskFeeLeaderboardSnapshots(context.Background(), db, now); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	snapshots, err := GetDelegationTaskFeeLeaderboard(context.Background(), db)
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(snapshots) != DelegationTaskFeeLeaderboardSize {
		t.Fatalf("expected %d rows, got %d", DelegationTaskFeeLeaderboardSize, len(snapshots))
	}
	if snapshots[0].DelegatorAddress != "0xdelegator-11" {
		t.Fatalf("unexpected top delegator %s", snapshots[0].DelegatorAddress)
	}
	if snapshots[9].DelegatorAddress != "0xdelegator-02" {
		t.Fatalf("unexpected last delegator %s", snapshots[9].DelegatorAddress)
	}
}

func TestRebuildDelegationTaskFeeLeaderboardExcludesSlashedAndOtherDays(t *testing.T) {
	db := setupDelegationTaskFeeLeaderboardTestDB(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	day := now.Truncate(24 * time.Hour)
	previousDay := day.Add(-24 * time.Hour)

	createDelegationLeaderboardFixture(t, db, "0xdelegator-a", "0xnode-a", 100, 1000, day)
	createDelegationLeaderboardFixture(t, db, "0xdelegator-b", "0xnode-b", 500, 2000, day)
	if err := db.Model(&models.Delegation{}).Where("delegator_address = ?", "0xdelegator-b").Update("slashed", true).Error; err != nil {
		t.Fatalf("mark delegation slashed: %v", err)
	}
	createDelegationLeaderboardFixture(t, db, "0xdelegator-c", "0xnode-c", 900, 3000, previousDay)
	if err := db.Create(&models.UserStakingEarning{
		UserAddress: "0xdelegator-a",
		NodeAddress: "0xnode-a",
		Network:     "base",
		Earning:     models.BigInt{Int: *big.NewInt(10000)},
		Time:        sql.NullTime{Valid: false},
	}).Error; err != nil {
		t.Fatalf("create total earning row: %v", err)
	}

	if err := RebuildDelegationTaskFeeLeaderboardSnapshots(context.Background(), db, now); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	snapshots, err := GetDelegationTaskFeeLeaderboard(context.Background(), db)
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 row, got %d", len(snapshots))
	}
	if snapshots[0].DelegatorAddress != "0xdelegator-a" {
		t.Fatalf("unexpected delegator %s", snapshots[0].DelegatorAddress)
	}
	if snapshots[0].TaskFee.String() != "100" {
		t.Fatalf("unexpected task fee %s", snapshots[0].TaskFee.String())
	}
}

func TestRebuildDelegationTaskFeeLeaderboardReplacesPreviousSnapshot(t *testing.T) {
	db := setupDelegationTaskFeeLeaderboardTestDB(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	day := now.Truncate(24 * time.Hour)

	createDelegationLeaderboardFixture(t, db, "0xdelegator-a", "0xnode-a", 100, 1000, day)
	if err := RebuildDelegationTaskFeeLeaderboardSnapshots(context.Background(), db, now); err != nil {
		t.Fatalf("first rebuild: %v", err)
	}

	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.UserStakingEarning{}).Error; err != nil {
		t.Fatalf("clear earnings: %v", err)
	}
	createDelegationLeaderboardFixture(t, db, "0xdelegator-b", "0xnode-b", 200, 2000, day)
	if err := RebuildDelegationTaskFeeLeaderboardSnapshots(context.Background(), db, now); err != nil {
		t.Fatalf("second rebuild: %v", err)
	}

	snapshots, err := GetDelegationTaskFeeLeaderboard(context.Background(), db)
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 row, got %d", len(snapshots))
	}
	if snapshots[0].DelegatorAddress != "0xdelegator-b" {
		t.Fatalf("unexpected delegator %s", snapshots[0].DelegatorAddress)
	}
}
