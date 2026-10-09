package service

import (
	"context"
	"crynux_relay/models"
	"testing"
	"time"

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
	if err := db.Create(&models.LoadedModel{ModelID: "qwen/qwen3.6-7b", Variant: "", ModelType: models.LoadedModelTypeLLM, MinVRAM: 24}).Error; err != nil {
		t.Fatalf("failed to create loaded model: %v", err)
	}

	updateLoadedModels(
		&models.InferenceTask{
			TaskType: models.TaskTypeLLM,
			ModelIDs: models.StringArray{"base:qwen/qwen3.6-7b", "base:meta/llama", "base:meta/llama+fp16", "lora:crynux-network/mylora"},
		},
		&models.Node{GPUVram: 16},
	)

	pending := loadedModelCache.take()
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending loaded models, got %d", len(pending))
	}
	if pending[loadedModelKey{ModelID: "meta/llama", Variant: ""}] != (pendingLoadedModel{ModelType: models.LoadedModelTypeLLM, MinVRAM: 16}) {
		t.Fatalf("unexpected pending meta/llama entry: %+v", pending[loadedModelKey{ModelID: "meta/llama", Variant: ""}])
	}
	if pending[loadedModelKey{ModelID: "meta/llama", Variant: "fp16"}] != (pendingLoadedModel{ModelType: models.LoadedModelTypeLLM, MinVRAM: 16}) {
		t.Fatalf("unexpected pending meta/llama+fp16 entry: %+v", pending[loadedModelKey{ModelID: "meta/llama", Variant: "fp16"}])
	}
	if pending[loadedModelKey{ModelID: "qwen/qwen3.6-7b", Variant: ""}] != (pendingLoadedModel{ModelType: models.LoadedModelTypeLLM, MinVRAM: 16}) {
		t.Fatalf("unexpected pending qwen entry: %+v", pending[loadedModelKey{ModelID: "qwen/qwen3.6-7b", Variant: ""}])
	}
	loadedModelCache.merge(pending)

	var dbLoadedModels []models.LoadedModel
	if err := db.Order("model_id ASC, variant ASC").Find(&dbLoadedModels).Error; err != nil {
		t.Fatalf("failed to query db loaded models: %v", err)
	}
	if len(dbLoadedModels) != 1 || dbLoadedModels[0].MinVRAM != 24 {
		t.Fatalf("expected db to remain unchanged before flush, got %+v", dbLoadedModels)
	}

	flushLoadedModelCache(context.Background(), db)

	dbLoadedModels = nil
	if err := db.Order("model_id ASC, variant ASC").Find(&dbLoadedModels).Error; err != nil {
		t.Fatalf("failed to query db loaded models after flush: %v", err)
	}
	if len(dbLoadedModels) != 3 {
		t.Fatalf("expected 3 db loaded models after flush, got %d", len(dbLoadedModels))
	}
}

func TestLoadedModelNodeCountCache(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeModel{}); err != nil {
		t.Fatalf("failed to migrate node models: %v", err)
	}
	nodeModels := []models.NodeModel{
		models.NewNodeModel("0x1", "base:qwen/qwen3.6-7b", true),
		models.NewNodeModel("0x1", "base:qwen/qwen3.6-7b+fp16", false),
		models.NewNodeModel("0x2", "base:qwen/qwen3.6-7b", false),
		models.NewNodeModel("0x2", "base:meta/llama", true),
		models.NewNodeModel("0x2", "lora:crynux-network/mylora", false),
	}
	if err := db.Create(&nodeModels).Error; err != nil {
		t.Fatalf("failed to create node models: %v", err)
	}

	cache := &baseDispatchNodeCountCache{}
	now := time.Now()
	counts, err := cache.get(context.Background(), db, now)
	if err != nil {
		t.Fatalf("failed to get node counts: %v", err)
	}
	if len(counts) != 3 {
		t.Fatalf("expected node counts for 3 dispatch ids, got %d", len(counts))
	}
	if counts["base:qwen/qwen3.6-7b"] != (models.HFModelNodeCount{OnDisk: 2, InMemory: 1}) {
		t.Fatalf("unexpected qwen node counts: %+v", counts["base:qwen/qwen3.6-7b"])
	}
	if counts["base:qwen/qwen3.6-7b+fp16"] != (models.HFModelNodeCount{OnDisk: 1, InMemory: 0}) {
		t.Fatalf("unexpected qwen fp16 node counts: %+v", counts["base:qwen/qwen3.6-7b+fp16"])
	}
	if counts["base:meta/llama"] != (models.HFModelNodeCount{OnDisk: 1, InMemory: 1}) {
		t.Fatalf("unexpected llama node counts: %+v", counts["base:meta/llama"])
	}

	if err := db.Where("node_address = ?", "0x2").Delete(&models.NodeModel{}).Error; err != nil {
		t.Fatalf("failed to delete node models: %v", err)
	}

	cachedCounts, err := cache.get(context.Background(), db, now.Add(loadedModelNodeCountTTL/2))
	if err != nil {
		t.Fatalf("failed to get cached node counts: %v", err)
	}
	if cachedCounts["base:qwen/qwen3.6-7b"] != (models.HFModelNodeCount{OnDisk: 2, InMemory: 1}) {
		t.Fatalf("expected cached counts within TTL, got %+v", cachedCounts["base:qwen/qwen3.6-7b"])
	}

	refreshedCounts, err := cache.get(context.Background(), db, now.Add(loadedModelNodeCountTTL))
	if err != nil {
		t.Fatalf("failed to get refreshed node counts: %v", err)
	}
	if len(refreshedCounts) != 2 {
		t.Fatalf("expected node counts for 2 dispatch ids after refresh, got %d", len(refreshedCounts))
	}
	if refreshedCounts["base:qwen/qwen3.6-7b"] != (models.HFModelNodeCount{OnDisk: 1, InMemory: 1}) {
		t.Fatalf("unexpected refreshed qwen node counts: %+v", refreshedCounts["base:qwen/qwen3.6-7b"])
	}
}
