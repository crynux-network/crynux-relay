package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"crynux_relay/config"
	"crynux_relay/models"

	"gorm.io/gorm"
)

func TestNodeStartTaskUpdatesOnlyReportedBaseModels(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}, &models.NodeModel{}); err != nil {
		t.Fatalf("failed to migrate node tables: %v", err)
	}

	node := models.Node{
		Address: "0xnode",
		Status:  models.NodeStatusAvailable,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	node.Models = []models.NodeModel{
		models.NewNodeModel(node.Address, "base:model-a", false),
		models.NewNodeModel(node.Address, "base:old-model", true),
		models.NewNodeModel(node.Address, "lora:reported-adapter", true),
	}
	if err := models.CreateNodeModels(context.Background(), db, node.Models); err != nil {
		t.Fatalf("create reported models: %v", err)
	}

	err := nodeStartTask(
		context.Background(),
		db,
		&node,
		"0xtask",
		[]string{"base:model-a", "base:missing", "lora:adapter"},
	)
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	stored, err := models.GetNodeModelsByNodeAddress(context.Background(), db, node.Address)
	if err != nil {
		t.Fatalf("load node models: %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("expected task start not to create model rows, got %d", len(stored))
	}
	inUse := make(map[string]bool, len(stored))
	for _, model := range stored {
		inUse[model.ModelID] = model.InUse
	}
	if !inUse["base:model-a"] {
		t.Fatal("expected the reported task base model to be in use")
	}
	if inUse["base:old-model"] {
		t.Fatal("expected the previous base model not to be in use")
	}
	if !inUse["lora:reported-adapter"] {
		t.Fatal("task start must not modify reported auxiliary model rows")
	}
	if _, ok := inUse["base:missing"]; ok {
		t.Fatal("missing base model must not be created by task start")
	}
	if _, ok := inUse["lora:adapter"]; ok {
		t.Fatal("auxiliary model must not be tracked by task start")
	}
}

func TestNodeStartTaskRejectsActiveHealthExclusion(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}, &models.NodeModel{}); err != nil {
		t.Fatalf("failed to migrate node tables: %v", err)
	}
	node := models.Node{
		Address:         "0xexcluded-node",
		Status:          models.NodeStatusAvailable,
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	err := nodeStartTask(t.Context(), db, &node, "0xtask", nil)
	if !errors.Is(err, ErrNodeHealthExcluded) {
		t.Fatalf("expected health exclusion error, got %v", err)
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusAvailable || stored.CurrentTaskIDCommitment.Valid || !stored.HealthExcluded {
		t.Fatal("expected rejected start to leave node available and excluded")
	}
}

func TestNodeStartTaskClearsRecoveredExclusionAtomically(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}, &models.NodeModel{}); err != nil {
		t.Fatalf("failed to migrate node tables: %v", err)
	}
	node := models.Node{
		Address:         "0xrecovered-node",
		Status:          models.NodeStatusAvailable,
		HealthBase:      config.GetConfig().QoS.HealthExcludeExitThreshold,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := nodeStartTask(t.Context(), db, &node, "0xtask", nil); err != nil {
		t.Fatalf("start task: %v", err)
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusBusy ||
		!stored.CurrentTaskIDCommitment.Valid ||
		stored.CurrentTaskIDCommitment.String != "0xtask" ||
		stored.HealthExcluded {
		t.Fatal("expected busy state, current task, and exclusion clear to commit together")
	}
}

func TestNodeStartTaskRollsBackRecoveredExclusionClear(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}, &models.NodeModel{}); err != nil {
		t.Fatalf("failed to migrate node tables: %v", err)
	}
	node := models.Node{
		Address:         "0xrollback-node",
		Status:          models.NodeStatusAvailable,
		HealthBase:      config.GetConfig().QoS.HealthExcludeExitThreshold,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	model := models.NewNodeModel(node.Address, "base:model-a", false)
	if err := model.Save(t.Context(), db); err != nil {
		t.Fatalf("create model: %v", err)
	}
	node.Models = []models.NodeModel{model}

	callbackName := "test:fail_node_model_update"
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

	if err := nodeStartTask(t.Context(), db, &node, "0xtask", []string{"base:model-a"}); err == nil {
		t.Fatal("expected task start failure")
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusAvailable || stored.CurrentTaskIDCommitment.Valid || !stored.HealthExcluded {
		t.Fatal("expected failed transaction to roll back node start and exclusion clear")
	}
}

func TestNodeFinishTaskKeepsHealthExcludedNodeAvailable(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("failed to migrate node table: %v", err)
	}
	node := models.Node{
		Address:                 "0xfinished-excluded-node",
		Status:                  models.NodeStatusBusy,
		CurrentTaskIDCommitment: sql.NullString{String: "0xtask", Valid: true},
		HealthBase:              0.1,
		HealthUpdatedAt:         sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:          true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := nodeFinishTask(t.Context(), db, &node); err != nil {
		t.Fatalf("finish task: %v", err)
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusAvailable || stored.CurrentTaskIDCommitment.Valid || !stored.HealthExcluded {
		t.Fatal("expected short-term exclusion to preserve Available status after task finish")
	}
}
