package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"math/big"
	"testing"

	"gorm.io/gorm"
)

func TestAbortNodeCurrentTaskForSlash(t *testing.T) {
	initServiceTestConfig(t)
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}, &models.RelayAccountEvent{}, &models.Event{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	const (
		nodeAddress = "0xnode"
		creator     = "0xcreator"
		commitment  = "0xtask"
	)
	taskFee := big.NewInt(100)
	task := &models.InferenceTask{
		TaskIDCommitment: commitment,
		Creator:          creator,
		Status:           models.TaskStarted,
		TaskType:         models.TaskTypeLLM,
		SelectedNode:     nodeAddress,
		TaskFee:          models.BigInt{Int: *taskFee},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	node := &models.Node{
		Address:                 nodeAddress,
		Status:                  models.NodeStatusBusy,
		CurrentTaskIDCommitment: sql.NullString{String: commitment, Valid: true},
	}
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
	})

	var postCommit func() error
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		postCommit, err = abortNodeCurrentTaskForSlash(ctx, tx, node)
		return err
	}); err != nil {
		t.Fatalf("failed to abort current task: %v", err)
	}
	if postCommit == nil {
		t.Fatal("expected post-commit task cleanup")
	}
	if err := postCommit(); err != nil {
		t.Fatalf("failed to apply post-commit task cleanup: %v", err)
	}

	var storedTask models.InferenceTask
	if err := db.First(&storedTask, task.ID).Error; err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	if storedTask.Status != models.TaskEndAborted {
		t.Fatalf("expected task to be aborted, got %d", storedTask.Status)
	}
	if storedTask.AbortReason != models.TaskAbortNodeSlashed {
		t.Fatalf("expected node-slashed abort reason, got %d", storedTask.AbortReason)
	}

	var refundCount int64
	if err := db.Model(&models.RelayAccountEvent{}).
		Where("type = ? AND address = ?", models.RelayAccountEventTypeTaskRefund, creator).
		Count(&refundCount).Error; err != nil {
		t.Fatalf("failed to count refund events: %v", err)
	}
	if refundCount != 1 {
		t.Fatalf("expected one refund event, got %d", refundCount)
	}
	var abortEventCount int64
	if err := db.Model(&models.Event{}).
		Where("type = ? AND task_id_commitment = ?", "TaskEndAborted", commitment).
		Count(&abortEventCount).Error; err != nil {
		t.Fatalf("failed to count abort events: %v", err)
	}
	if abortEventCount != 1 {
		t.Fatalf("expected one abort event, got %d", abortEventCount)
	}

	relayAccountCache.mu.RLock()
	refundedBalance := new(big.Int).Set(relayAccountCache.accounts[creator])
	relayAccountCache.mu.RUnlock()
	if refundedBalance.Cmp(taskFee) != 0 {
		t.Fatalf("expected refunded balance %s, got %s", taskFee, refundedBalance)
	}
}

func TestAbortNodeCurrentTaskForSlashLeavesTerminalTaskUnchanged(t *testing.T) {
	initServiceTestConfig(t)
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	task := &models.InferenceTask{
		TaskIDCommitment: "0xinvalidated",
		Status:           models.TaskEndInvalidated,
		SelectedNode:     "0xnode",
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	node := &models.Node{
		Address:                 task.SelectedNode,
		Status:                  models.NodeStatusBusy,
		CurrentTaskIDCommitment: sql.NullString{String: task.TaskIDCommitment, Valid: true},
	}

	postCommit, err := abortNodeCurrentTaskForSlash(ctx, db, node)
	if err != nil {
		t.Fatalf("terminal task check failed: %v", err)
	}
	if postCommit != nil {
		t.Fatal("terminal task must not be aborted again")
	}
}

func TestAbortNodeCurrentTaskForSlashClearsGroupRefundSnapshots(t *testing.T) {
	initTaskPricingTestStore(t)
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}, &models.RelayAccountEvent{}, &models.Event{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	const (
		nodeAddress   = "0xgroup-node"
		creator       = "0xgroup-creator"
		taskID        = "group-task-id"
		validatedCommit = "validated-commitment"
		refundCommit    = "refund-commitment"
	)
	taskFee := big.NewInt(100)
	validated := &models.InferenceTask{
		TaskIDCommitment: validatedCommit,
		TaskID:           taskID,
		Creator:          creator,
		Status:           models.TaskGroupValidated,
		TaskType:         models.TaskTypeLLM,
		SelectedNode:     nodeAddress,
		TaskFee:          models.BigInt{Int: *taskFee},
	}
	refund := &models.InferenceTask{
		TaskIDCommitment: refundCommit,
		TaskID:           taskID,
		Creator:          creator,
		Status:           models.TaskEndGroupRefund,
		TaskType:         models.TaskTypeLLM,
		SelectedNode:     "0xrefund-node",
		TaskFee:          models.BigInt{Int: *taskFee},
	}
	if err := db.Create(validated).Error; err != nil {
		t.Fatalf("failed to create validated task: %v", err)
	}
	if err := db.Create(refund).Error; err != nil {
		t.Fatalf("failed to create refund task: %v", err)
	}

	CaptureTaskExecutionGPUSnapshot(validatedCommit, "A100", 40)
	CaptureTaskExecutionGPUSnapshot(refundCommit, "A100", 40)

	node := &models.Node{
		Address:                 nodeAddress,
		Status:                  models.NodeStatusBusy,
		CurrentTaskIDCommitment: sql.NullString{String: validatedCommit, Valid: true},
	}
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
		DeleteTaskExecutionGPUSnapshot(validatedCommit)
		DeleteTaskExecutionGPUSnapshot(refundCommit)
	})

	var postCommit func() error
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		postCommit, err = abortNodeCurrentTaskForSlash(ctx, tx, node)
		return err
	}); err != nil {
		t.Fatalf("failed to abort group-validated task: %v", err)
	}
	if postCommit == nil {
		t.Fatal("expected post-commit task cleanup")
	}
	if err := postCommit(); err != nil {
		t.Fatalf("failed to apply post-commit task cleanup: %v", err)
	}

	if _, _, ok := TaskExecutionGPUSnapshot(validatedCommit); ok {
		t.Fatal("expected validated task snapshot removed")
	}
	if _, _, ok := TaskExecutionGPUSnapshot(refundCommit); ok {
		t.Fatal("expected group-refund snapshot removed when group can no longer upload")
	}
}
