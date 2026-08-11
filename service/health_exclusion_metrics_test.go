package service

import (
	"context"
	"database/sql"
	"math/big"
	"testing"
	"time"

	"crynux_relay/blockchain/bindings"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func nodeEventCounterValue(t *testing.T, event string) float64 {
	t.Helper()
	value := &dto.Metric{}
	metric := metrics.NodeEvents.WithLabelValues(event)
	if err := metric.(prometheus.Metric).Write(value); err != nil {
		t.Fatalf("write node event metric %s: %v", event, err)
	}
	return value.GetCounter().GetValue()
}

func createHealthExclusionTimeoutFixture(t *testing.T, healthBase float64, alreadyExcluded bool) (*models.InferenceTask, *models.Node) {
	t.Helper()
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}, &models.Node{}, &models.RelayAccountEvent{}, &models.Event{}); err != nil {
		t.Fatalf("migrate timeout tables: %v", err)
	}

	const (
		nodeAddress = "0xhealth-timeout-node"
		creator     = "0xhealth-timeout-creator"
		commitment  = "0xhealth-timeout-task"
	)
	taskFee := big.NewInt(100)
	task := &models.InferenceTask{
		TaskIDCommitment: commitment,
		Creator:          creator,
		Status:           models.TaskStarted,
		TaskType:         models.TaskTypeLLM,
		SelectedNode:     nodeAddress,
		AbortReason:      models.TaskAbortTimeout,
		TaskFee:          models.BigInt{Int: *taskFee},
		StartTime:        sql.NullTime{Time: time.Now().UTC().Add(-2 * time.Minute), Valid: true},
		Timeout:          60,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	node := &models.Node{
		Address:                 nodeAddress,
		Status:                  models.NodeStatusBusy,
		CurrentTaskIDCommitment: sql.NullString{String: commitment, Valid: true},
		HealthBase:              healthBase,
		HealthUpdatedAt:         sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:          alreadyExcluded,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
	})
	return task, node
}

func TestSetTaskStatusEndAbortedIncrementsHealthExcludedEvent(t *testing.T) {
	task, _ := createHealthExclusionTimeoutFixture(t, 0.5, false)
	db := config.GetDB()

	beforeExcluded := nodeEventCounterValue(t, "health_excluded")
	beforeRecovered := nodeEventCounterValue(t, "health_recovered")
	if err := SetTaskStatusEndAborted(t.Context(), db, task, "relay"); err != nil {
		t.Fatalf("abort task: %v", err)
	}
	if got := nodeEventCounterValue(t, "health_excluded") - beforeExcluded; got != 1 {
		t.Fatalf("expected health_excluded +1, got %g", got)
	}
	if got := nodeEventCounterValue(t, "health_recovered") - beforeRecovered; got != 0 {
		t.Fatalf("expected health_recovered unchanged, got delta %g", got)
	}

	var storedNode models.Node
	if err := db.Where("address = ?", task.SelectedNode).First(&storedNode).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if !storedNode.HealthExcluded {
		t.Fatal("expected node to be health excluded after timeout")
	}
}

func TestSetTaskStatusEndAbortedDoesNotIncrementHealthExcludedWithoutNewExclusion(t *testing.T) {
	task, _ := createHealthExclusionTimeoutFixture(t, 1.0, false)
	db := config.GetDB()

	beforeExcluded := nodeEventCounterValue(t, "health_excluded")
	if err := SetTaskStatusEndAborted(t.Context(), db, task, "relay"); err != nil {
		t.Fatalf("abort task: %v", err)
	}
	if got := nodeEventCounterValue(t, "health_excluded") - beforeExcluded; got != 0 {
		t.Fatalf("expected health_excluded unchanged when penalty does not enter exclusion, got delta %g", got)
	}

	var storedNode models.Node
	if err := db.Where("address = ?", task.SelectedNode).First(&storedNode).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if storedNode.HealthExcluded {
		t.Fatal("expected node to remain not health excluded")
	}
}

func TestSetTaskStatusStartedIncrementsHealthRecoveredEvent(t *testing.T) {
	task, node := createHealthExclusionTaskStartFixture(t, true)
	db := config.GetDB()

	beforeRecovered := nodeEventCounterValue(t, "health_recovered")
	beforeExcluded := nodeEventCounterValue(t, "health_excluded")
	if err := SetTaskStatusStarted(t.Context(), db, task, node); err != nil {
		t.Fatalf("start task: %v", err)
	}
	if got := nodeEventCounterValue(t, "health_recovered") - beforeRecovered; got != 1 {
		t.Fatalf("expected health_recovered +1, got %g", got)
	}
	if got := nodeEventCounterValue(t, "health_excluded") - beforeExcluded; got != 0 {
		t.Fatalf("expected health_excluded unchanged, got delta %g", got)
	}
}

func TestSetNodeStatusJoinDoesNotIncrementHealthRecovered(t *testing.T) {
	ctx := context.Background()
	db := setupBlockchainProcessorTestDB(t)
	if err := db.AutoMigrate(&models.DelegatedStakingNodeListSnapshot{}); err != nil {
		t.Fatalf("migrate delegated staking node list snapshots: %v", err)
	}
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000DD")
	node := seedTestNode(t, db, nodeAddress.Hex(), "network-b", models.NodeStatusQuit, 10)
	if err := db.Model(&node).Updates(map[string]interface{}{
		"health_excluded": true,
		"health_base":     0.1,
	}).Error; err != nil {
		t.Fatalf("set health exclusion on quit node: %v", err)
	}
	node.HealthExcluded = true
	node.HealthBase = 0.1

	originalGetStakingInfo := getStakingInfo
	originalGetNodeDelegatorShare := getNodeDelegatorShare
	originalGetNodeStakingInfos := getNodeStakingInfos
	t.Cleanup(func() {
		getStakingInfo = originalGetStakingInfo
		getNodeDelegatorShare = originalGetNodeDelegatorShare
		getNodeStakingInfos = originalGetNodeStakingInfos
	})
	getStakingInfo = func(ctx context.Context, address common.Address, network string) (bindings.NodeStakingStakingInfo, error) {
		return bindings.NodeStakingStakingInfo{
			StakedBalance: big.NewInt(10),
		}, nil
	}
	getNodeDelegatorShare = func(ctx context.Context, address common.Address, network string) (uint8, error) {
		return 0, nil
	}
	getNodeStakingInfos = func(ctx context.Context, address common.Address, network string) ([]common.Address, []*big.Int, error) {
		return nil, nil, nil
	}

	beforeJoin := nodeEventCounterValue(t, "join")
	beforeRecovered := nodeEventCounterValue(t, "health_recovered")
	if err := SetNodeStatusJoin(ctx, db, &node, []string{"model-a"}); err != nil {
		t.Fatalf("SetNodeStatusJoin failed: %v", err)
	}
	if got := nodeEventCounterValue(t, "join") - beforeJoin; got != 1 {
		t.Fatalf("expected join +1, got %g", got)
	}
	if got := nodeEventCounterValue(t, "health_recovered") - beforeRecovered; got != 0 {
		t.Fatalf("expected join reset not to increment health_recovered, got delta %g", got)
	}
	if node.HealthExcluded {
		t.Fatal("expected join to clear health exclusion")
	}
}
