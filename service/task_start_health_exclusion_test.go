package service

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"crynux_relay/config"
	"crynux_relay/models"

	"gorm.io/gorm"
)

func createHealthExclusionTaskStartFixture(t *testing.T, recovered bool) (*models.InferenceTask, *models.Node) {
	t.Helper()
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}, &models.Node{}, &models.NodeModel{}, &models.Event{}); err != nil {
		t.Fatalf("migrate task start tables: %v", err)
	}
	task := &models.InferenceTask{
		TaskIDCommitment: "0xhealth-start-task",
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeSDFTLora,
		TaskVersion:      "1.0.0",
		Timeout:          60,
		CreateTime:       sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	healthBase := 0.5
	if recovered {
		healthBase = config.GetConfig().QoS.HealthExcludeExitThreshold
	}
	node := &models.Node{
		Address:         "0xhealth-start-node",
		Status:          models.NodeStatusAvailable,
		MajorVersion:    1,
		HealthBase:      healthBase,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return task, node
}

func TestSetTaskStatusStartedRejectsActiveHealthExclusion(t *testing.T) {
	task, node := createHealthExclusionTaskStartFixture(t, false)
	db := config.GetDB()

	err := SetTaskStatusStarted(t.Context(), db, task, node)
	if !errors.Is(err, ErrNodeHealthExcluded) {
		t.Fatalf("expected health exclusion error, got %v", err)
	}
	var storedTask models.InferenceTask
	if err := db.First(&storedTask, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	var storedNode models.Node
	if err := db.First(&storedNode, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if storedTask.Status != models.TaskQueued ||
		storedNode.Status != models.NodeStatusAvailable ||
		storedNode.CurrentTaskIDCommitment.Valid ||
		!storedNode.HealthExcluded {
		t.Fatal("expected rejected start to preserve queued task and available excluded node")
	}
}

func TestSetTaskStatusStartedClearsRecoveredExclusionWithTaskStart(t *testing.T) {
	task, node := createHealthExclusionTaskStartFixture(t, true)
	db := config.GetDB()

	if err := SetTaskStatusStarted(t.Context(), db, task, node); err != nil {
		t.Fatalf("start task: %v", err)
	}
	var storedTask models.InferenceTask
	if err := db.First(&storedTask, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	var storedNode models.Node
	if err := db.First(&storedNode, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if storedTask.Status != models.TaskStarted ||
		storedNode.Status != models.NodeStatusBusy ||
		!storedNode.CurrentTaskIDCommitment.Valid ||
		storedNode.CurrentTaskIDCommitment.String != task.TaskIDCommitment ||
		storedNode.HealthExcluded {
		t.Fatal("expected task start, busy node, current task, and exclusion clear to commit together")
	}
}

func TestSetTaskStatusStartedRollsBackTaskAndExclusionClear(t *testing.T) {
	task, node := createHealthExclusionTaskStartFixture(t, true)
	db := config.GetDB()
	model := models.NewNodeModel(node.Address, "base:model-a", false)
	if err := model.Save(t.Context(), db); err != nil {
		t.Fatalf("create model: %v", err)
	}
	node.Models = []models.NodeModel{model}
	task.ModelIDs = models.StringArray{"base:model-a"}
	if err := db.Model(task).Update("model_ids", task.ModelIDs).Error; err != nil {
		t.Fatalf("update task models: %v", err)
	}

	callbackName := "test:fail_task_start_node_model_update"
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "NodeModel" {
			tx.AddError(errors.New("forced node model update failure"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		db.Callback().Update().Remove(callbackName)
	})

	if err := SetTaskStatusStarted(t.Context(), db, task, node); err == nil {
		t.Fatal("expected task start failure")
	}
	var storedTask models.InferenceTask
	if err := db.First(&storedTask, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	var storedNode models.Node
	if err := db.First(&storedNode, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if storedTask.Status != models.TaskQueued ||
		storedNode.Status != models.NodeStatusAvailable ||
		storedNode.CurrentTaskIDCommitment.Valid ||
		!storedNode.HealthExcluded {
		t.Fatal("expected failed start to roll back task, node, and exclusion changes")
	}
}
