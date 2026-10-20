package service

import (
	"context"
	"crynux_relay/models"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLoadedModelTask(t *testing.T, repository, revision, dtype string) *models.InferenceTask {
	t.Helper()
	exact := models.EncodeHashList([]models.Hash32{testModelFileHash(t, repository, revision)})
	cacheHash := models.Hash32{1}
	return &models.InferenceTask{
		TaskType:                 models.TaskTypeLLM,
		TaskArgs:                 `{"quantization_config":{"bits":4}}`,
		ModelName:                repository,
		ExecutionDType:           dtype,
		ExecutionModelFileHashes: exact,
		RequestedModelCacheHash:  cacheHash.Bytes(),
	}
}

func TestLoadedModelCacheMergeAndFlush(t *testing.T) {
	loadedModelCache = newLoadedModelMinVRAMCache()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.LoadedModel{}); err != nil {
		t.Fatalf("failed to migrate loaded models: %v", err)
	}

	task := newLoadedModelTask(t, "qwen/qwen3.6-7b", "rev-1", "float16")
	updateLoadedModels(task, &models.Node{GPUVram: 24})
	updateLoadedModels(task, &models.Node{GPUVram: 16})
	updateLoadedModels(newLoadedModelTask(t, "qwen/qwen3.6-7b", "rev-1", "bfloat16"), &models.Node{GPUVram: 16})
	updateLoadedModels(newLoadedModelTask(t, "qwen/qwen3.6-7b", "rev-2", "float16"), &models.Node{GPUVram: 32})
	updateLoadedModels(&models.InferenceTask{TaskType: models.TaskTypeLLM}, &models.Node{GPUVram: 8})

	pending := loadedModelCache.take()
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending loaded models, got %d", len(pending))
	}
	executionHashes, err := task.ExecutionModelFileHashList()
	if err != nil {
		t.Fatalf("failed to decode execution hashes: %v", err)
	}
	key := loadedModelKey{
		RequiredModelFilesHash: models.RequiredModelFilesHash(executionHashes),
		ModelCacheHash:         models.Hash32{1},
		ExecutionDType:         "float16",
	}
	if got := pending[key]; got.MinVRAM != 16 || got.ModelID != "qwen/qwen3.6-7b" || got.ModelType != models.LoadedModelTypeLLM {
		t.Fatalf("unexpected pending entry: %+v", got)
	}
	loadedModelCache.merge(pending)

	var dbLoadedModels []models.LoadedModel
	if err := db.Find(&dbLoadedModels).Error; err != nil {
		t.Fatalf("failed to query db loaded models: %v", err)
	}
	if len(dbLoadedModels) != 0 {
		t.Fatalf("expected db to remain unchanged before flush, got %+v", dbLoadedModels)
	}

	flushLoadedModelCache(context.Background(), db)

	dbLoadedModels = nil
	if err := db.Find(&dbLoadedModels).Error; err != nil {
		t.Fatalf("failed to query db loaded models after flush: %v", err)
	}
	if len(dbLoadedModels) != 3 {
		t.Fatalf("expected 3 db loaded models after flush, got %d", len(dbLoadedModels))
	}
	for _, loadedModel := range dbLoadedModels {
		if string(loadedModel.QuantizationConfig) != `{"bits":4}` {
			t.Fatalf("unexpected quantization config: %s", loadedModel.QuantizationConfig)
		}
		members, err := loadedModel.MemberHashes()
		if err != nil || len(members) != 1 {
			t.Fatalf("unexpected member hashes: %v, %v", members, err)
		}
	}
}

func TestLoadedModelViewCache(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.LoadedModel{}); err != nil {
		t.Fatalf("failed to migrate loaded models: %v", err)
	}
	migrateModelStateTables(t, db)

	fileA := testModelFile(t, "meta/llama", "rev-a")
	fileB := testModelFile(t, "meta/llama", "rev-b")
	seedNodeModelFiles(t, db, "0x1", fileA)
	seedNodeModelFiles(t, db, "0x2", fileA, fileB)
	hashA := testModelFileHash(t, "meta/llama", "rev-a")

	cacheHash := models.Hash32{7}
	loadedModel := models.LoadedModel{
		RequiredModelFilesHash:  models.RequiredModelFilesHash([]models.Hash32{hashA}).Bytes(),
		ModelCacheHash:          cacheHash.Bytes(),
		ExecutionDType:          "float16",
		RequiredModelFileHashes: models.EncodeHashList([]models.Hash32{hashA}),
		QuantizationConfig:      models.JSONText(`null`),
		ModelID:                 "meta/llama",
		ModelType:               models.LoadedModelTypeLLM,
		MinVRAM:                 16,
	}
	if err := db.Create(&loadedModel).Error; err != nil {
		t.Fatalf("failed to create loaded model: %v", err)
	}
	memory := models.NewNodeInMemoryModel("0x1", cacheHash, []models.Hash32{hashA}, []byte(`{}`), "float16")
	if err := db.Create(&memory).Error; err != nil {
		t.Fatalf("failed to create in-memory model: %v", err)
	}
	otherDType := models.NewNodeInMemoryModel("0x2", cacheHash, []models.Hash32{hashA}, []byte(`{}`), "bfloat16")
	if err := db.Create(&otherDType).Error; err != nil {
		t.Fatalf("failed to create in-memory model: %v", err)
	}

	cache := &loadedModelViewCacheStore{}
	now := time.Now()
	views, err := cache.get(context.Background(), db, now)
	if err != nil {
		t.Fatalf("failed to get loaded model views: %v", err)
	}
	if len(views) != 1 || views[0].OnDiskNodeCount != 2 || views[0].InMemoryNodeCount != 1 {
		t.Fatalf("unexpected views: %+v", views)
	}

	if err := db.Where("node_address = ?", "0x2").Delete(&models.NodeModel{}).Error; err != nil {
		t.Fatalf("failed to delete node models: %v", err)
	}
	cached, err := cache.get(context.Background(), db, now.Add(loadedModelNodeCountTTL/2))
	if err != nil {
		t.Fatalf("failed to get cached views: %v", err)
	}
	if cached[0].OnDiskNodeCount != 2 {
		t.Fatalf("expected cached counts within TTL, got %+v", cached[0])
	}

	refreshed, err := cache.get(context.Background(), db, now.Add(loadedModelNodeCountTTL))
	if err != nil {
		t.Fatalf("failed to get refreshed views: %v", err)
	}
	if refreshed[0].OnDiskNodeCount != 1 {
		t.Fatalf("unexpected refreshed counts: %+v", refreshed[0])
	}
}
