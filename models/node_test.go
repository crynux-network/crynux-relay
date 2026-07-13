package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNewNodeModelNormalizesIDs(t *testing.T) {
	nodeModel := NewNodeModel("0x1", "base:Qwen/Qwen3-8B+FP16", true)
	if nodeModel.ModelID != "base:qwen/qwen3-8b+fp16" {
		t.Fatalf("unexpected model id: %q", nodeModel.ModelID)
	}
	if nodeModel.HFModelID != "qwen/qwen3-8b" {
		t.Fatalf("unexpected hf model id: %q", nodeModel.HFModelID)
	}

	loraModel := NewNodeModel("0x1", "lora:Crynux-Network/MyLora", false)
	if loraModel.ModelID != "lora:crynux-network/mylora" || loraModel.HFModelID != "" {
		t.Fatalf("unexpected lora model: %+v", loraModel)
	}
}

func TestNodeModelBeforeSaveNormalizesIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&NodeModel{}); err != nil {
		t.Fatalf("failed to migrate node models: %v", err)
	}

	nodeModel := NodeModel{NodeAddress: "0x1", ModelID: "base:Qwen/Qwen3-8B"}
	if err := db.Create(&nodeModel).Error; err != nil {
		t.Fatalf("failed to create node model: %v", err)
	}

	var stored NodeModel
	if err := db.First(&stored, nodeModel.ID).Error; err != nil {
		t.Fatalf("failed to load node model: %v", err)
	}
	if stored.ModelID != "base:qwen/qwen3-8b" {
		t.Fatalf("unexpected stored model id: %q", stored.ModelID)
	}
	if stored.HFModelID != "qwen/qwen3-8b" {
		t.Fatalf("unexpected stored hf model id: %q", stored.HFModelID)
	}

	stored.ModelID = "base:Meta/Llama"
	if err := db.Save(&stored).Error; err != nil {
		t.Fatalf("failed to save node model: %v", err)
	}
	var resaved NodeModel
	if err := db.First(&resaved, nodeModel.ID).Error; err != nil {
		t.Fatalf("failed to reload node model: %v", err)
	}
	if resaved.ModelID != "base:meta/llama" || resaved.HFModelID != "meta/llama" {
		t.Fatalf("unexpected resaved node model: %+v", resaved)
	}
}
