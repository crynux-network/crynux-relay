package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"math"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDelegationAPRTestTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.VestingRecord{}, &models.VestingDelegationEmissionDetail{}, &models.NodeEarning{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE node_stakings (
	id integer PRIMARY KEY AUTOINCREMENT,
	created_at datetime,
	updated_at datetime,
	deleted_at datetime,
	node_address text NOT NULL,
	operator_staking text NOT NULL,
	delegator_staking text NOT NULL,
	time datetime NOT NULL
)`).Error; err != nil {
		t.Fatalf("create node_stakings: %v", err)
	}
}

func TestBuildDelegationAPRRangeUsesConfiguredStartTime(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	start, end, err := buildDelegationAPRRange(now, "2026-07-01T08:30:00+08:00")
	if err != nil {
		t.Fatalf("build APR range: %v", err)
	}

	expectedStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(expectedStart) {
		t.Fatalf("unexpected start %s", start)
	}
	if !end.Equal(now) {
		t.Fatalf("unexpected end %s", end)
	}
}

func TestBuildDelegatedStakingNodeListSnapshotCalculatesSortFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	setupDelegationAPRTestTables(t, db)

	network := "base"
	nodeAddress := "0xnode"
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	mainnetStart := "2026-01-01T00:00:00Z"
	globalDelegationCaches = map[string]*delegationCache{
		network: {
			nodeDelegations: map[string]map[string]*big.Int{},
			userDelegations: map[string]map[string]*big.Int{},
			userStakeAmount: map[string]*big.Int{},
			nodeStakeAmount: map[string]*big.Int{},
		},
		"near": {
			nodeDelegations: map[string]map[string]*big.Int{},
			userDelegations: map[string]map[string]*big.Int{},
			userStakeAmount: map[string]*big.Int{},
			nodeStakeAmount: map[string]*big.Int{},
		},
	}
	UpdateDelegation("0xdelegator", nodeAddress, big.NewInt(30), network)
	UpdateDelegation("0xinactive", nodeAddress, big.NewInt(70), "near")
	globalNodeVestingStakeCache = newNodeVestingStakeCache()
	globalNodeVestingStakeCache.set(nodeAddress, []models.VestingRecord{
		{
			Address:      nodeAddress,
			TotalAmount:  models.BigInt{Int: *big.NewInt(20)},
			StartTime:    now,
			DurationDays: 180,
			Status:       models.VestingStatusActive,
			Slashed:      false,
		},
	})
	globalMaxStaking = newMaxStaking()
	UpdateMaxStaking(nodeAddress, big.NewInt(150))

	records := []models.VestingRecord{
		{
			Address:        nodeAddress,
			TotalAmount:    models.BigInt{Int: *big.NewInt(7)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			DurationDays:   180,
			Type:           models.VestingTypeNode,
			AdminSignature: "signature",
			Status:         models.VestingStatusActive,
		},
		{
			Address:        nodeAddress,
			TotalAmount:    models.BigInt{Int: *big.NewInt(11)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			DurationDays:   180,
			Type:           models.VestingTypeDelegation,
			AdminSignature: "signature",
			Status:         models.VestingStatusActive,
		},
		{
			Address:        "0xdelegator",
			TotalAmount:    models.BigInt{Int: *big.NewInt(285)},
			ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
			StartTime:      time.Date(2026, 1, 29, 0, 0, 0, 0, time.UTC),
			DurationDays:   180,
			Type:           models.VestingTypeDelegation,
			AdminSignature: "signature",
			Status:         models.VestingStatusActive,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create vesting records: %v", err)
	}
	if err := db.Create(&models.VestingDelegationEmissionDetail{
		VestingRecordID: records[2].ID,
		UserAddress:     "0xdelegator",
		NodeAddress:     nodeAddress,
		Network:         network,
		TaskFee:         models.BigInt{Int: *big.NewInt(30)},
		EmissionAmount:  models.BigInt{Int: *big.NewInt(285)},
		StartTime:       time.Date(2026, 1, 29, 0, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatalf("create vesting delegation emission detail: %v", err)
	}
	earningTime := now.Add(-24 * time.Hour)
	if err := db.Create(&models.NodeEarning{
		NodeAddress:      nodeAddress,
		OperatorEarning:  models.BigInt{Int: *big.NewInt(1)},
		DelegatorEarning: models.BigInt{Int: *big.NewInt(15)},
		Time:             sql.NullTime{Time: earningTime, Valid: true},
	}).Error; err != nil {
		t.Fatalf("create node earning: %v", err)
	}
	if err := db.Create(&models.NodeStaking{
		NodeAddress:      nodeAddress,
		OperatorStaking:  models.BigInt{Int: *big.NewInt(100)},
		DelegatorStaking: models.BigInt{Int: *big.NewInt(300)},
		Time:             earningTime,
	}).Error; err != nil {
		t.Fatalf("create node staking: %v", err)
	}

	snapshot, err := BuildDelegatedStakingNodeListSnapshot(context.Background(), db, models.Node{
		Network:        network,
		Address:        nodeAddress,
		Status:         models.NodeStatusAvailable,
		GPUName:        "RTX 4090",
		GPUVram:        24,
		MajorVersion:   1,
		MinorVersion:   2,
		PatchVersion:   3,
		StakeAmount:    models.BigInt{Int: *big.NewInt(100)},
		DelegatorShare: 10,
		HealthBase:     1,
	}, now, mainnetStart)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if snapshot.OperatorEmission4w.Int.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("unexpected operator emission %s", snapshot.OperatorEmission4w.String())
	}
	if snapshot.DelegatorEmission4w.Int.Cmp(big.NewInt(285)) != 0 {
		t.Fatalf("unexpected delegator emission %s", snapshot.DelegatorEmission4w.String())
	}
	if snapshot.OperatorStaking.Int.Cmp(big.NewInt(120)) != 0 {
		t.Fatalf("unexpected operator staking %s", snapshot.OperatorStaking.String())
	}
	if snapshot.DelegatorStaking.Int.Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("unexpected delegator staking %s", snapshot.DelegatorStaking.String())
	}
	if snapshot.TotalStaking.Int.Cmp(big.NewInt(150)) != 0 {
		t.Fatalf("unexpected total staking %s", snapshot.TotalStaking.String())
	}
	if snapshot.DelegatorsNum != 1 {
		t.Fatalf("unexpected delegator count %d", snapshot.DelegatorsNum)
	}
	if snapshot.StatusGroup != models.DelegatedStakingNodeStatusGroupRunning || snapshot.StatusRank != 0 {
		t.Fatalf("unexpected status group/rank %s/%d", snapshot.StatusGroup, snapshot.StatusRank)
	}
	if math.Abs(snapshot.DelegationApr12m-365) > 0.000001 {
		t.Fatalf("unexpected APR %f", snapshot.DelegationApr12m)
	}
	if snapshot.AprObservationDays != 1 {
		t.Fatalf("unexpected APR observation days %d", snapshot.AprObservationDays)
	}
	if !snapshot.DelegationAprUpdatedAt.Equal(now.UTC()) {
		t.Fatalf("unexpected APR updated at %s", snapshot.DelegationAprUpdatedAt)
	}
}

func TestCalculateNodeDelegationAPR12mReturnsZeroForZeroDenominator(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	setupDelegationAPRTestTables(t, db)

	nodeAddress := "0xnode"
	now := time.Date(2026, 1, 29, 12, 0, 0, 0, time.UTC)
	t1 := now.Add(-24 * time.Hour)
	if err := db.Create(&models.NodeEarning{
		NodeAddress:      nodeAddress,
		OperatorEarning:  models.BigInt{Int: *big.NewInt(0)},
		DelegatorEarning: models.BigInt{Int: *big.NewInt(50)},
		Time:             sql.NullTime{Time: t1, Valid: true},
	}).Error; err != nil {
		t.Fatalf("create node earning: %v", err)
	}
	if err := db.Create(&models.NodeStaking{
		NodeAddress:      nodeAddress,
		OperatorStaking:  models.BigInt{Int: *big.NewInt(100)},
		DelegatorStaking: models.BigInt{Int: *big.NewInt(0)},
		Time:             t1,
	}).Error; err != nil {
		t.Fatalf("create node staking: %v", err)
	}

	apr, observationDays, err := CalculateNodeDelegationAPR12m(context.Background(), db, nodeAddress, now)
	if err != nil {
		t.Fatalf("calculate APR: %v", err)
	}
	if apr != 0 {
		t.Fatalf("expected zero APR, got %f", apr)
	}
	if observationDays != 1 {
		t.Fatalf("unexpected observation days %d", observationDays)
	}
}
