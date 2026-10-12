package incentive

import (
	"context"
	"crynux_relay/models"
	"crynux_relay/utils"
	"database/sql"
	"fmt"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTopNodesByTotalTaskFeeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeEarning{}, &models.NodeIncentive{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func createNodeEarningFixture(t *testing.T, db *gorm.DB, nodeAddress string, operatorEarning, delegatorEarning int64, day time.Time) {
	t.Helper()
	if err := db.Create(&models.NodeEarning{
		NodeAddress:      nodeAddress,
		OperatorEarning:  models.BigInt{Int: *big.NewInt(operatorEarning)},
		DelegatorEarning: models.BigInt{Int: *big.NewInt(delegatorEarning)},
		Time:             sql.NullTime{Time: day, Valid: true},
	}).Error; err != nil {
		t.Fatalf("create node earning: %v", err)
	}
}

func createNodeIncentiveFixture(t *testing.T, db *gorm.DB, nodeAddress string, taskCount, sdCount, llmCount, sdFTCount int64, day time.Time) {
	t.Helper()
	if err := db.Create(&models.NodeIncentive{
		NodeAddress:       nodeAddress,
		Incentive:         0,
		Time:              day,
		TaskCount:         taskCount,
		SDTaskCount:       sdCount,
		LLMTaskCount:      llmCount,
		SDFTLoraTaskCount: sdFTCount,
	}).Error; err != nil {
		t.Fatalf("create node incentive: %v", err)
	}
}

func TestLoadTopNodesByTotalTaskFeeEmpty(t *testing.T) {
	db := setupTopNodesByTotalTaskFeeTestDB(t)
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	start := now.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	nodes, err := loadTopNodesByTotalTaskFee(context.Background(), db, start, end, topNodeIncentiveSize)
	if err != nil {
		t.Fatalf("load top nodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected empty result, got %d", len(nodes))
	}
}

func TestLoadTopNodesByTotalTaskFeeRanksByOperatorPlusDelegator(t *testing.T) {
	db := setupTopNodesByTotalTaskFeeTestDB(t)
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	start := now.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	// Operator-only node with higher operator fee than the high-delegation node.
	createNodeEarningFixture(t, db, "0xnode-operator-high", 50_000_000_000_000_000, 0, start)
	createNodeIncentiveFixture(t, db, "0xnode-operator-high", 10, 4, 6, 0, start)

	// High total from delegator share; operator portion is tiny.
	createNodeEarningFixture(t, db, "0xnode-delegator-high", 1_000_000_000_000_000, 100_000_000_000_000_000, start)
	createNodeIncentiveFixture(t, db, "0xnode-delegator-high", 178, 0, 178, 0, start)

	createNodeEarningFixture(t, db, "0xnode-mid", 20_000_000_000_000_000, 5_000_000_000_000_000, start)
	createNodeIncentiveFixture(t, db, "0xnode-mid", 30, 10, 20, 0, start)

	// Other-day and total rows MUST NOT affect ranking.
	createNodeEarningFixture(t, db, "0xnode-yesterday", 900_000_000_000_000_000, 0, start.Add(-24*time.Hour))
	if err := db.Create(&models.NodeEarning{
		NodeAddress:      "0xnode-total",
		OperatorEarning:  models.BigInt{Int: *big.NewInt(900_000_000_000_000_000)},
		DelegatorEarning: models.BigInt{Int: *big.NewInt(0)},
		Time:             sql.NullTime{Valid: false},
	}).Error; err != nil {
		t.Fatalf("create total node earning: %v", err)
	}

	nodes, err := loadTopNodesByTotalTaskFee(context.Background(), db, start, end, topNodeIncentiveSize)
	if err != nil {
		t.Fatalf("load top nodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if nodes[0].NodeAddress != "0xnode-delegator-high" {
		t.Fatalf("expected first node 0xnode-delegator-high, got %s", nodes[0].NodeAddress)
	}
	if nodes[1].NodeAddress != "0xnode-operator-high" {
		t.Fatalf("expected second node 0xnode-operator-high, got %s", nodes[1].NodeAddress)
	}
	if nodes[2].NodeAddress != "0xnode-mid" {
		t.Fatalf("expected third node 0xnode-mid, got %s", nodes[2].NodeAddress)
	}

	expectedIncentive, _ := utils.WeiToEther(big.NewInt(101_000_000_000_000_000)).Float64()
	if nodes[0].Incentive != expectedIncentive {
		t.Fatalf("expected first incentive %v, got %v", expectedIncentive, nodes[0].Incentive)
	}
	if nodes[0].TaskCount != 178 || nodes[0].LLMTaskCount != 178 {
		t.Fatalf("unexpected task counters for first node: %+v", nodes[0])
	}
	if nodes[1].TaskCount != 10 || nodes[1].SDTaskCount != 4 || nodes[1].LLMTaskCount != 6 {
		t.Fatalf("unexpected task counters for second node: %+v", nodes[1])
	}
}

func TestLoadTopNodesByTotalTaskFeeLimitsToTopTen(t *testing.T) {
	db := setupTopNodesByTotalTaskFeeTestDB(t)
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	start := now.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	for i := 0; i < 12; i++ {
		addr := fmt.Sprintf("0xnode-%02d", i)
		createNodeEarningFixture(t, db, addr, int64(1000+i), 0, start)
		createNodeIncentiveFixture(t, db, addr, int64(i+1), 0, 0, 0, start)
	}

	nodes, err := loadTopNodesByTotalTaskFee(context.Background(), db, start, end, topNodeIncentiveSize)
	if err != nil {
		t.Fatalf("load top nodes: %v", err)
	}
	if len(nodes) != topNodeIncentiveSize {
		t.Fatalf("expected %d nodes, got %d", topNodeIncentiveSize, len(nodes))
	}
	if nodes[0].NodeAddress != "0xnode-11" {
		t.Fatalf("expected first node 0xnode-11, got %s", nodes[0].NodeAddress)
	}
	if nodes[9].NodeAddress != "0xnode-02" {
		t.Fatalf("expected tenth node 0xnode-02, got %s", nodes[9].NodeAddress)
	}
}
