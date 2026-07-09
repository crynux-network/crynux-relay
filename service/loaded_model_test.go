package service

import (
	"context"
	"crynux_relay/models"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadedModelCacheMergeAndFlush(t *testing.T) {
	loadedModelCache = newLoadedModelMinVRAMCache()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.LoadedModel{}); err != nil {
		t.Fatalf("failed to migrate loaded models: %v", err)
	}
	if err := db.Create(&models.LoadedModel{ModelID: "qwen/qwen3.6-7b", MinVRAM: 24}).Error; err != nil {
		t.Fatalf("failed to create loaded model: %v", err)
	}

	updateLoadedModels(
		&models.InferenceTask{ModelIDs: models.StringArray{"qwen/qwen3.6-7b", "meta/llama", "meta/llama"}},
		&models.Node{GPUVram: 16},
	)

	pending := loadedModelCache.take()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending loaded models, got %d", len(pending))
	}
	if pending["meta/llama"] != 16 {
		t.Fatalf("unexpected pending meta/llama min vram: %d", pending["meta/llama"])
	}
	if pending["qwen/qwen3.6-7b"] != 16 {
		t.Fatalf("unexpected pending qwen min vram: %d", pending["qwen/qwen3.6-7b"])
	}
	loadedModelCache.merge(pending)

	var dbLoadedModels []models.LoadedModel
	if err := db.Order("model_id ASC").Find(&dbLoadedModels).Error; err != nil {
		t.Fatalf("failed to query db loaded models: %v", err)
	}
	if len(dbLoadedModels) != 1 || dbLoadedModels[0].MinVRAM != 24 {
		t.Fatalf("expected db to remain unchanged before flush, got %+v", dbLoadedModels)
	}

	flushLoadedModelCache(context.Background(), db)

	dbLoadedModels = nil
	if err := db.Order("model_id ASC").Find(&dbLoadedModels).Error; err != nil {
		t.Fatalf("failed to query db loaded models after flush: %v", err)
	}
	if len(dbLoadedModels) != 2 {
		t.Fatalf("expected 2 db loaded models after flush, got %d", len(dbLoadedModels))
	}
	if dbLoadedModels[0].ModelID != "meta/llama" || dbLoadedModels[0].MinVRAM != 16 {
		t.Fatalf("unexpected first db loaded model: %+v", dbLoadedModels[0])
	}
	if dbLoadedModels[1].ModelID != "qwen/qwen3.6-7b" || dbLoadedModels[1].MinVRAM != 16 {
		t.Fatalf("unexpected second db loaded model: %+v", dbLoadedModels[1])
	}
}
