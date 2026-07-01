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
	if err := db.AutoMigrate(&models.NodeEarning{}, &models.UserEarning{}, &models.UserStakingEarning{}); err != nil {
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
