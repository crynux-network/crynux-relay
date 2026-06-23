package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminKickoutAllNodesAndAbortTasks(t *testing.T) {
	ctx := context.Background()
	db := newAdminMigrationTestDB(t)
	resetAdminMigrationTestCaches()

	creator := "0x00000000000000000000000000000000000000c1"
	tasks := []models.InferenceTask{
		newAdminMigrationTask("queued", creator, models.TaskQueued, "", big.NewInt(100)),
		newAdminMigrationTask("started", creator, models.TaskStarted, "node-a", big.NewInt(200)),
		newAdminMigrationTask("done", creator, models.TaskEndSuccess, "node-b", big.NewInt(300)),
	}
	for i := range tasks {
		if err := db.Create(&tasks[i]).Error; err != nil {
			t.Fatalf("failed to seed task: %v", err)
		}
	}
	nodes := []models.Node{
		newAdminMigrationNode("node-a", models.NodeStatusBusy, "started"),
		newAdminMigrationNode("node-b", models.NodeStatusAvailable, ""),
		newAdminMigrationNode("node-c", models.NodeStatusPaused, ""),
	}
	for i := range nodes {
		if err := db.Create(&nodes[i]).Error; err != nil {
			t.Fatalf("failed to seed node: %v", err)
		}
		if err := db.Create(&models.NodeModel{NodeAddress: nodes[i].Address, ModelID: "model"}).Error; err != nil {
			t.Fatalf("failed to seed node model: %v", err)
		}
	}
	if err := db.Create(&models.NetworkNodeData{Address: "node-a"}).Error; err != nil {
		t.Fatalf("failed to seed network node data: %v", err)
	}
	if err := db.Create(&models.NodeNameCount{GPUName: "A100", GPUVram: 40, NodeVersion: "1.2.3", ActiveCount: 2}).Error; err != nil {
		t.Fatalf("failed to seed node name count: %v", err)
	}
	if err := db.Create(&models.BlockchainCursor{Network: "crynux-on-base", LastBlockNum: 100, LastUpdateTime: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed target cursor: %v", err)
	}
	if err := db.Create(&models.BlockchainCursor{Network: "base", LastBlockNum: 100, LastUpdateTime: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed other cursor: %v", err)
	}

	result, err := AdminKickoutAllNodesAndAbortTasks(ctx, db, "crynux-on-base", "admin")
	if err != nil {
		t.Fatalf("migration reset failed: %v", err)
	}
	if result.AbortedTaskCount != 2 {
		t.Fatalf("expected 2 aborted tasks, got %d", result.AbortedTaskCount)
	}
	if result.KickedOutNodeCount != 3 {
		t.Fatalf("expected 3 kicked out nodes, got %d", result.KickedOutNodeCount)
	}
	if result.DeletedCursorCount != 1 {
		t.Fatalf("expected 1 deleted cursor, got %d", result.DeletedCursorCount)
	}

	for _, taskIDCommitment := range []string{"queued", "started"} {
		var task models.InferenceTask
		if err := db.First(&task, "task_id_commitment = ?", taskIDCommitment).Error; err != nil {
			t.Fatalf("failed to load task %s: %v", taskIDCommitment, err)
		}
		if task.Status != models.TaskEndAborted {
			t.Fatalf("expected task %s to be aborted, got %d", taskIDCommitment, task.Status)
		}
		if task.AbortReason != models.TaskAbortReasonNone {
			t.Fatalf("expected task %s abort reason to stay default, got %d", taskIDCommitment, task.AbortReason)
		}
		if !task.ValidatedTime.Valid {
			t.Fatalf("expected task %s validated time to be set", taskIDCommitment)
		}
	}

	var finishedTask models.InferenceTask
	if err := db.First(&finishedTask, "task_id_commitment = ?", "done").Error; err != nil {
		t.Fatalf("failed to load finished task: %v", err)
	}
	if finishedTask.Status != models.TaskEndSuccess {
		t.Fatalf("expected finished task status to stay success, got %d", finishedTask.Status)
	}

	var activeNodeCount int64
	if err := db.Model(&models.Node{}).Where("status <> ?", models.NodeStatusQuit).Count(&activeNodeCount).Error; err != nil {
		t.Fatalf("failed to count active nodes: %v", err)
	}
	if activeNodeCount != 0 {
		t.Fatalf("expected no active nodes, got %d", activeNodeCount)
	}
	var nodesWithCurrentTask int64
	if err := db.Model(&models.Node{}).Where("current_task_id_commitment IS NOT NULL").Count(&nodesWithCurrentTask).Error; err != nil {
		t.Fatalf("failed to count nodes with current task: %v", err)
	}
	if nodesWithCurrentTask != 0 {
		t.Fatalf("expected no node current task, got %d", nodesWithCurrentTask)
	}

	assertTableCount(t, db, &models.NodeModel{}, 0)
	assertTableCount(t, db, &models.NetworkNodeData{}, 0)
	assertTableCount(t, db, &models.NodeNameCount{}, 0)
	assertCursorExists(t, db, "crynux-on-base", false)
	assertCursorExists(t, db, "base", true)

	var refundEventCount int64
	if err := db.Model(&models.RelayAccountEvent{}).Where("type = ?", models.RelayAccountEventTypeTaskRefund).Count(&refundEventCount).Error; err != nil {
		t.Fatalf("failed to count refund events: %v", err)
	}
	if refundEventCount != 2 {
		t.Fatalf("expected 2 refund events, got %d", refundEventCount)
	}
	var taskAbortEventCount int64
	if err := db.Model(&models.Event{}).Where("type = ?", "TaskEndAborted").Count(&taskAbortEventCount).Error; err != nil {
		t.Fatalf("failed to count task abort events: %v", err)
	}
	if taskAbortEventCount != 2 {
		t.Fatalf("expected 2 task abort events, got %d", taskAbortEventCount)
	}
	var nodeKickoutEventCount int64
	if err := db.Model(&models.Event{}).Where("type = ?", "NodeKickedOut").Count(&nodeKickoutEventCount).Error; err != nil {
		t.Fatalf("failed to count node kickout events: %v", err)
	}
	if nodeKickoutEventCount != 3 {
		t.Fatalf("expected 3 node kickout events, got %d", nodeKickoutEventCount)
	}
}

func TestAdminKickoutAllNodesAndAbortTasksRequiresNetwork(t *testing.T) {
	_, err := AdminKickoutAllNodesAndAbortTasks(context.Background(), newAdminMigrationTestDB(t), "", "admin")
	if !errors.Is(err, ErrMigrationNetworkRequired) {
		t.Fatalf("expected ErrMigrationNetworkRequired, got %v", err)
	}
}

func newAdminMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.InferenceTask{},
		&models.Node{},
		&models.NodeModel{},
		&models.NetworkNodeData{},
		&models.NodeNameCount{},
		&models.BlockchainCursor{},
		&models.Event{},
		&models.RelayAccountEvent{},
		&models.RelayAccount{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func resetAdminMigrationTestCaches() {
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = make(map[string]*big.Int)
	relayAccountCache.mu.Unlock()
	resetNodeNamePolicyCacheForTest()
	globalMaxStaking = newMaxStaking()
}

func newAdminMigrationTask(taskIDCommitment, creator string, status models.TaskStatus, selectedNode string, fee *big.Int) models.InferenceTask {
	return models.InferenceTask{
		TaskIDCommitment: taskIDCommitment,
		Creator:          creator,
		Status:           status,
		TaskVersion:      "1.2.3",
		TaskFee:          models.BigInt{Int: *fee},
		SelectedNode:     selectedNode,
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
		StartTime:        sql.NullTime{Time: time.Now(), Valid: selectedNode != ""},
	}
}

func newAdminMigrationNode(address string, status models.NodeStatus, taskIDCommitment string) models.Node {
	currentTask := sql.NullString{Valid: false}
	if taskIDCommitment != "" {
		currentTask = sql.NullString{String: taskIDCommitment, Valid: true}
	}
	return models.Node{
		Address:                 address,
		Network:                 "crynux-on-base",
		Status:                  status,
		GPUName:                 "A100",
		GPUVram:                 40,
		MajorVersion:            1,
		MinorVersion:            2,
		PatchVersion:            3,
		StakeAmount:             models.BigInt{Int: *big.NewInt(500)},
		CurrentTaskIDCommitment: currentTask,
	}
}

func assertTableCount(t *testing.T, db *gorm.DB, model interface{}, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		t.Fatalf("failed to count table: %v", err)
	}
	if count != expected {
		t.Fatalf("expected table count %d, got %d", expected, count)
	}
}

func assertCursorExists(t *testing.T, db *gorm.DB, network string, expected bool) {
	t.Helper()
	var count int64
	if err := db.Model(&models.BlockchainCursor{}).Where("network = ?", network).Count(&count).Error; err != nil {
		t.Fatalf("failed to count cursor: %v", err)
	}
	if (count > 0) != expected {
		t.Fatalf("expected cursor %s existence %t, got count %d", network, expected, count)
	}
}
