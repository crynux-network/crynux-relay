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

func TestBuildDelegatedStakingNodeListSnapshotCalculatesSortFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.VestingRecord{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

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
	}
	UpdateDelegation("0xdelegator", nodeAddress, big.NewInt(30), network)
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
			Source:         "test",
			ExternalID:     "node-1",
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
			Source:         "test",
			ExternalID:     "delegation-1",
			AdminSignature: "signature",
			Status:         models.VestingStatusActive,
		},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create vesting records: %v", err)
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
}
