package service

import (
	"context"
	"testing"

	"crynux_relay/config"
	"crynux_relay/models"
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
