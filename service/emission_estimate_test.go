package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func sqlNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

func setupEmissionEstimateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeEarning{}, &models.UserEarning{}, &models.UserStakingEarning{}, &models.DelegatedStakingNodeListSnapshot{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestRefreshCurrentEmissionEstimateSnapshotAggregatesCurrentWeek(t *testing.T) {
	db := setupEmissionEstimateTestDB(t)
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	currentWeekDay := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	previousWeekDay := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)

	nodeEarnings := []models.NodeEarning{
		{
			NodeAddress:      "0xnode-a",
			OperatorEarning:  models.BigInt{Int: *big.NewInt(20)},
			DelegatorEarning: models.BigInt{Int: *big.NewInt(30)},
			Time:             sqlNullTime(currentWeekDay),
		},
		{
			NodeAddress:      "0xnode-b",
			OperatorEarning:  models.BigInt{Int: *big.NewInt(10)},
			DelegatorEarning: models.BigInt{Int: *big.NewInt(0)},
			Time:             sqlNullTime(currentWeekDay),
		},
		{
			NodeAddress:      "0xnode-a",
			OperatorEarning:  models.BigInt{Int: *big.NewInt(1000)},
			DelegatorEarning: models.BigInt{Int: *big.NewInt(1000)},
			Time:             sqlNullTime(previousWeekDay),
		},
	}
	if err := db.Create(&nodeEarnings).Error; err != nil {
		t.Fatalf("create node earnings: %v", err)
	}

	userEarnings := []models.UserEarning{
		{
			UserAddress: "0xuser-a",
			Earning:     models.BigInt{Int: *big.NewInt(30)},
			Time:        sqlNullTime(currentWeekDay),
		},
		{
			UserAddress: "0xuser-b",
			Earning:     models.BigInt{Int: *big.NewInt(10)},
			Time:        sqlNullTime(currentWeekDay),
		},
		{
			UserAddress: "0xuser-a",
			Earning:     models.BigInt{Int: *big.NewInt(1000)},
			Time:        sqlNullTime(previousWeekDay),
		},
	}
	if err := db.Create(&userEarnings).Error; err != nil {
		t.Fatalf("create user earnings: %v", err)
	}

	userStakingEarnings := []models.UserStakingEarning{
		{
			UserAddress: "0xuser-a",
			NodeAddress: "0xnode-a",
			Network:     "base",
			Earning:     models.BigInt{Int: *big.NewInt(25)},
			Time:        sqlNullTime(currentWeekDay),
		},
	}
	if err := db.Create(&userStakingEarnings).Error; err != nil {
		t.Fatalf("create user staking earnings: %v", err)
	}

	if err := RefreshCurrentEmissionEstimateSnapshot(context.Background(), db, now, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("refresh estimate snapshot: %v", err)
	}

	pool := big.NewInt(9280212)
	totalTaskFee := big.NewInt(70)
	assertEstimate := func(name string, got EmissionEstimateResult, taskFee int64) {
		t.Helper()
		expected := big.NewInt(0).Div(big.NewInt(0).Mul(big.NewInt(taskFee), pool), totalTaskFee)
		if got.EstimatedEmission.Cmp(expected) != 0 {
			t.Fatalf("%s expected %s, got %s", name, expected, got.EstimatedEmission)
		}
		if got.EmissionWeekStart != time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC).Unix() {
			t.Fatalf("%s unexpected week start %d", name, got.EmissionWeekStart)
		}
		if got.EmissionWeekEnd != time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC).Unix() {
			t.Fatalf("%s unexpected week end %d", name, got.EmissionWeekEnd)
		}
		if got.EstimateUpdatedAt != now.Unix() {
			t.Fatalf("%s unexpected updated_at %d", name, got.EstimateUpdatedAt)
		}
	}

	assertEstimate("operator", GetNodeOperatorEmissionEstimate("0xnode-a"), 20)
	assertEstimate("node delegations", GetNodeDelegationEmissionEstimate("0xnode-a"), 30)
	assertEstimate("user delegations", GetUserDelegationEmissionEstimate("0xuser-a"), 30)
	assertEstimate("single delegation", GetSingleDelegationEmissionEstimate("0xuser-a", "0xnode-a", "base"), 25)

	zero := GetSingleDelegationEmissionEstimate("0xuser-a", "0xnode-a", "near")
	if zero.EstimatedEmission.Sign() != 0 {
		t.Fatalf("expected zero estimate for missing delegation, got %s", zero.EstimatedEmission)
	}
}

func TestGetNodeDelegationWeeklyTaskFeeEstimateScalesByElapsedWeekTime(t *testing.T) {
	db := setupEmissionEstimateTestDB(t)
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	currentWeekDay := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)

	if err := db.Create(&models.NodeEarning{
		NodeAddress:      "0xnode-a",
		OperatorEarning:  models.BigInt{Int: *big.NewInt(20)},
		DelegatorEarning: models.BigInt{Int: *big.NewInt(30)},
		Time:             sqlNullTime(currentWeekDay),
	}).Error; err != nil {
		t.Fatalf("create node earning: %v", err)
	}

	if err := RefreshCurrentEmissionEstimateSnapshot(context.Background(), db, now, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("refresh estimate snapshot: %v", err)
	}

	// Week start is Jan 8, elapsed 2.5 days: 30 * 7 / 2.5 = 84.
	got := GetNodeDelegationWeeklyTaskFeeEstimate("0xnode-a")
	if got.Cmp(big.NewInt(84)) != 0 {
		t.Fatalf("expected 84, got %s", got)
	}

	missing := GetNodeDelegationWeeklyTaskFeeEstimate("0xnode-missing")
	if missing.Sign() != 0 {
		t.Fatalf("expected zero for missing node, got %s", missing)
	}
}

func TestGetNodeDelegationWeeklyTaskFeeEstimateClampsMinimumElapsed(t *testing.T) {
	db := setupEmissionEstimateTestDB(t)
	now := time.Date(2026, 1, 8, 1, 0, 0, 0, time.UTC)

	if err := db.Create(&models.NodeEarning{
		NodeAddress:      "0xnode-a",
		OperatorEarning:  models.BigInt{Int: *big.NewInt(20)},
		DelegatorEarning: models.BigInt{Int: *big.NewInt(30)},
		Time:             sqlNullTime(time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)),
	}).Error; err != nil {
		t.Fatalf("create node earning: %v", err)
	}

	if err := RefreshCurrentEmissionEstimateSnapshot(context.Background(), db, now, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("refresh estimate snapshot: %v", err)
	}

	// Elapsed 1 hour is clamped to 1 day: 30 * 7 / 1 = 210.
	got := GetNodeDelegationWeeklyTaskFeeEstimate("0xnode-a")
	if got.Cmp(big.NewInt(210)) != 0 {
		t.Fatalf("expected 210, got %s", got)
	}
}

func TestRefreshCurrentEmissionEstimateSnapshotReturnsZeroWithNoTaskFee(t *testing.T) {
	db := setupEmissionEstimateTestDB(t)
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	if err := RefreshCurrentEmissionEstimateSnapshot(context.Background(), db, now, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("refresh estimate snapshot: %v", err)
	}

	estimate := GetNodeOperatorEmissionEstimate("0xnode-a")
	if estimate.EstimatedEmission.Sign() != 0 {
		t.Fatalf("expected zero estimate, got %s", estimate.EstimatedEmission)
	}
	if estimate.EmissionWeekStart != time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("unexpected week start %d", estimate.EmissionWeekStart)
	}
}

func TestUpdateDelegatedStakingNodeListEmissionEstimates(t *testing.T) {
	db := setupEmissionEstimateTestDB(t)
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	currentWeekDay := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)

	nodeEarnings := []models.NodeEarning{
		{
			NodeAddress:      "0xnode-a",
			OperatorEarning:  models.BigInt{Int: *big.NewInt(20)},
			DelegatorEarning: models.BigInt{Int: *big.NewInt(30)},
			Time:             sqlNullTime(currentWeekDay),
		},
	}
	if err := db.Create(&nodeEarnings).Error; err != nil {
		t.Fatalf("create node earnings: %v", err)
	}
	userEarnings := []models.UserEarning{
		{
			UserAddress: "0xuser-a",
			Earning:     models.BigInt{Int: *big.NewInt(40)},
			Time:        sqlNullTime(currentWeekDay),
		},
	}
	if err := db.Create(&userEarnings).Error; err != nil {
		t.Fatalf("create user earnings: %v", err)
	}
	snapshots := []models.DelegatedStakingNodeListSnapshot{
		{NodeAddress: "0xnode-a", Network: "base", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 4090", GPUVram: 24, Version: "1.0.0", DelegationAprUpdatedAt: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)},
		{NodeAddress: "0xnode-b", Network: "base", StatusGroup: "running", StatusRank: 0, GPUName: "RTX 5090", GPUVram: 32, Version: "1.0.0", EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *big.NewInt(99)}, DelegationAprUpdatedAt: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	if err := RefreshCurrentEmissionEstimateSnapshot(context.Background(), db, now, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("refresh estimate snapshot: %v", err)
	}
	if err := UpdateDelegatedStakingNodeListEmissionEstimates(context.Background(), db); err != nil {
		t.Fatalf("update node list estimates: %v", err)
	}

	var nodeA models.DelegatedStakingNodeListSnapshot
	if err := db.Where("node_address = ?", "0xnode-a").First(&nodeA).Error; err != nil {
		t.Fatalf("load node-a snapshot: %v", err)
	}
	expected := big.NewInt(0).Div(big.NewInt(0).Mul(big.NewInt(30), big.NewInt(9280212)), big.NewInt(60))
	expectedOperator := big.NewInt(0).Div(big.NewInt(0).Mul(big.NewInt(20), big.NewInt(9280212)), big.NewInt(60))
	if nodeA.EstimatedUpcomingOperatorEmission.Int.Cmp(expectedOperator) != 0 {
		t.Fatalf("expected node-a operator estimate %s, got %s", expectedOperator, &nodeA.EstimatedUpcomingOperatorEmission.Int)
	}
	if nodeA.EstimatedUpcomingDelegatorEmission.Int.Cmp(expected) != 0 {
		t.Fatalf("expected node-a delegator estimate %s, got %s", expected, &nodeA.EstimatedUpcomingDelegatorEmission.Int)
	}

	var nodeB models.DelegatedStakingNodeListSnapshot
	if err := db.Where("node_address = ?", "0xnode-b").First(&nodeB).Error; err != nil {
		t.Fatalf("load node-b snapshot: %v", err)
	}
	if nodeB.EstimatedUpcomingOperatorEmission.Sign() != 0 || nodeB.EstimatedUpcomingDelegatorEmission.Sign() != 0 {
		t.Fatalf("expected node-b zero estimates, got operator %s delegator %s", &nodeB.EstimatedUpcomingOperatorEmission.Int, &nodeB.EstimatedUpcomingDelegatorEmission.Int)
	}
}
