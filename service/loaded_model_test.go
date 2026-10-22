package service

import (
	"context"
	"crynux_relay/models"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func llmLoadedModelArgs(repository, quantization string) string {
	return fmt.Sprintf(`{"version":"3.7.0","model":%q,"dtype":"auto","quantization_config":%s,"messages":[{"role":"user","content":"hi"}]}`, repository, quantization)
}

func imageLoadedModelArgs(componentRepository string) string {
	return fmt.Sprintf(`{"version":"3.7.0","base_model":{"name":"org/sd"},"dtype":"float16","components":{"transformer":{"format":"gguf","repo":%q,"format_kwargs":{"filename":"a.gguf"}}},"prompt":"a cat"}`, componentRepository)
}

func newLoadedModelTask(t *testing.T, taskArgs string, taskType models.TaskType, modelName, dtype string, files ...models.Hash32) *models.InferenceTask {
	t.Helper()
	requirements, err := models.DeriveTaskModelRequirements(taskArgs, taskType)
	if err != nil {
		t.Fatalf("failed to derive model requirements: %v", err)
	}
	task := &models.InferenceTask{
		TaskType:                 taskType,
		TaskArgs:                 taskArgs,
		ModelName:                modelName,
		ExecutionDType:           dtype,
		ExecutionModelFileHashes: models.EncodeHashList(files),
	}
	task.SetModelRequirements(requirements)
	return task
}

func newLoadedModelRow(t *testing.T, task *models.InferenceTask, minVRAM uint64) models.LoadedModel {
	t.Helper()
	files, err := task.ExecutionModelFileHashList()
	if err != nil {
		t.Fatalf("failed to decode execution hashes: %v", err)
	}
	config, err := task.RequestedModelCacheConfig()
	if err != nil {
		t.Fatalf("failed to derive model cache config: %v", err)
	}
	quantization, err := models.TaskQuantizationConfigJSON(task.TaskArgs)
	if err != nil {
		t.Fatalf("failed to derive quantization config: %v", err)
	}
	return models.LoadedModel{
		RequiredModelFilesHash:  models.RequiredModelFilesHash(files).Bytes(),
		ModelCacheHash:          task.RequestedModelCacheHash,
		ExecutionDType:          task.ExecutionDType,
		RequiredModelFileHashes: models.EncodeHashList(files),
		QuantizationConfig:      quantization,
		ModelCacheConfig:        config,
		ModelID:                 task.ModelName,
		ModelType:               models.LoadedModelTypeFromTaskType(task.TaskType),
		MinVRAM:                 minVRAM,
	}
}

func newLoadedModelDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.LoadedModel{}); err != nil {
		t.Fatalf("failed to migrate loaded models: %v", err)
	}
	migrateModelStateTables(t, db)
	return db
}

func TestLoadedModelCacheMergeAndFlush(t *testing.T) {
	loadedModelCache = newLoadedModelMinVRAMCache()
	db := newLoadedModelDB(t)

	args := llmLoadedModelArgs("qwen/qwen3.6-7b", `{"bits":4}`)
	hash1 := testModelFileHash(t, "qwen/qwen3.6-7b", "rev-1")
	hash2 := testModelFileHash(t, "qwen/qwen3.6-7b", "rev-2")
	task := newLoadedModelTask(t, args, models.TaskTypeLLM, "qwen/qwen3.6-7b", "float16", hash1)
	updateLoadedModels(task, &models.Node{GPUVram: 24})
	updateLoadedModels(task, &models.Node{GPUVram: 16})
	updateLoadedModels(newLoadedModelTask(t, args, models.TaskTypeLLM, "qwen/qwen3.6-7b", "bfloat16", hash1), &models.Node{GPUVram: 16})
	updateLoadedModels(newLoadedModelTask(t, args, models.TaskTypeLLM, "qwen/qwen3.6-7b", "float16", hash2), &models.Node{GPUVram: 32})
	updateLoadedModels(&models.InferenceTask{TaskType: models.TaskTypeLLM}, &models.Node{GPUVram: 8})

	mismatched := newLoadedModelTask(t, args, models.TaskTypeLLM, "qwen/qwen3.6-7b", "float16", hash1)
	mismatched.RequestedModelCacheHash = models.Hash32{9}.Bytes()
	updateLoadedModels(mismatched, &models.Node{GPUVram: 4})

	pending := loadedModelCache.take()
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending loaded models, got %d", len(pending))
	}
	cacheHash, err := task.RequestedModelCacheHash32()
	if err != nil {
		t.Fatalf("failed to decode cache hash: %v", err)
	}
	key := loadedModelKey{
		RequiredModelFilesHash: models.RequiredModelFilesHash([]models.Hash32{hash1}),
		ModelCacheHash:         cacheHash,
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
	wantConfig, err := task.RequestedModelCacheConfig()
	if err != nil {
		t.Fatalf("failed to derive model cache config: %v", err)
	}
	for _, loadedModel := range dbLoadedModels {
		if string(loadedModel.QuantizationConfig) != `{"bits":4}` {
			t.Fatalf("unexpected quantization config: %s", loadedModel.QuantizationConfig)
		}
		if string(loadedModel.ModelCacheConfig) != string(wantConfig) {
			t.Fatalf("unexpected model cache config: %s", loadedModel.ModelCacheConfig)
		}
		members, err := loadedModel.MemberHashes()
		if err != nil || len(members) != 1 {
			t.Fatalf("unexpected member hashes: %v, %v", members, err)
		}
	}
}

func TestUpsertLoadedModelKeepsIdentityConfig(t *testing.T) {
	db := newLoadedModelDB(t)
	task := newLoadedModelTask(t, llmLoadedModelArgs("org/llm", `null`), models.TaskTypeLLM, "org/llm", "float16", testModelFileHash(t, "org/llm", "rev-1"))
	row := newLoadedModelRow(t, task, 24)
	if err := models.UpsertLoadedModelMinVRAMs(context.Background(), db, []models.LoadedModel{row}); err != nil {
		t.Fatalf("failed to upsert loaded model: %v", err)
	}

	changed := row
	changed.ID = 0
	changed.ModelCacheConfig = models.JSONText(`{"other":true}`)
	changed.MinVRAM = 16
	if err := models.UpsertLoadedModelMinVRAMs(context.Background(), db, []models.LoadedModel{changed}); err != nil {
		t.Fatalf("failed to upsert loaded model again: %v", err)
	}

	var stored []models.LoadedModel
	if err := db.Find(&stored).Error; err != nil {
		t.Fatalf("failed to query loaded models: %v", err)
	}
	if len(stored) != 1 || stored[0].MinVRAM != 16 || string(stored[0].ModelCacheConfig) != string(row.ModelCacheConfig) {
		t.Fatalf("unexpected stored loaded models: %+v", stored)
	}
}

func TestLoadedModelViewCache(t *testing.T) {
	db := newLoadedModelDB(t)

	fileA := testModelFile(t, "meta/llama", "rev-a")
	fileB := testModelFile(t, "meta/llama", "rev-b")
	seedNodeModelFiles(t, db, "0x1", fileA)
	seedNodeModelFiles(t, db, "0x2", fileA, fileB)
	hashA := testModelFileHash(t, "meta/llama", "rev-a")

	task := newLoadedModelTask(t, llmLoadedModelArgs("meta/llama", `null`), models.TaskTypeLLM, "meta/llama", "float16", hashA)
	loadedModel := newLoadedModelRow(t, task, 16)
	if err := db.Create(&loadedModel).Error; err != nil {
		t.Fatalf("failed to create loaded model: %v", err)
	}
	cacheHash, err := task.RequestedModelCacheHash32()
	if err != nil {
		t.Fatalf("failed to decode cache hash: %v", err)
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

func TestBuildLoadedModelViewsAggregatesByCacheIdentity(t *testing.T) {
	db := newLoadedModelDB(t)

	hashA := testModelFileHash(t, "org/sd", "rev-a")
	hashB := testModelFileHash(t, "org/sd", "rev-b")
	seedNodeModelFiles(t, db, "0x1", testModelFile(t, "org/sd", "rev-a"))
	seedNodeModelFiles(t, db, "0x2", testModelFile(t, "org/sd", "rev-b"))
	seedNodeModelFiles(t, db, "0x3", testModelFile(t, "org/sd", "rev-a"), testModelFile(t, "org/sd", "rev-b"))
	seedNodeModelFiles(t, db, "0x4", testModelFile(t, "org/other", "rev-a"))

	taskA := newLoadedModelTask(t, imageLoadedModelArgs("org/comp-a"), models.TaskTypeSD, "org/sd", "float16", hashA)
	taskB := newLoadedModelTask(t, imageLoadedModelArgs("org/comp-a"), models.TaskTypeSD, "org/sd", "float16", hashB)
	taskOther := newLoadedModelTask(t, imageLoadedModelArgs("org/comp-b"), models.TaskTypeSD, "org/sd", "float16", hashA)
	if taskA.RequestedModelCacheHash == nil || string(taskA.RequestedModelCacheHash) != string(taskB.RequestedModelCacheHash) {
		t.Fatalf("revisions must share the cache hash")
	}
	if string(taskA.RequestedModelCacheHash) == string(taskOther.RequestedModelCacheHash) {
		t.Fatalf("different component repositories must not share the cache hash")
	}

	rows := []models.LoadedModel{
		newLoadedModelRow(t, taskA, 24),
		newLoadedModelRow(t, taskB, 16),
		newLoadedModelRow(t, taskOther, 32),
	}
	incomplete := newLoadedModelRow(t, taskA, 8)
	incomplete.ExecutionDType = "bfloat16"
	incomplete.ModelCacheConfig = models.JSONText("not json")
	mismatched := newLoadedModelRow(t, taskA, 8)
	mismatched.ExecutionDType = "float32"
	mismatched.ModelCacheConfig = models.JSONText(`{"other":true}`)
	rows = append(rows, incomplete, mismatched)
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to create loaded models: %v", err)
	}

	cacheHash, err := taskA.RequestedModelCacheHash32()
	if err != nil {
		t.Fatalf("failed to decode cache hash: %v", err)
	}
	memories := []models.NodeInMemoryModel{
		models.NewNodeInMemoryModel("0x1", cacheHash, []models.Hash32{hashA}, []byte(`{}`), "float16"),
		models.NewNodeInMemoryModel("0x3", cacheHash, []models.Hash32{hashA, hashB}, []byte(`{}`), "float16"),
	}
	if err := db.Create(&memories).Error; err != nil {
		t.Fatalf("failed to create in-memory models: %v", err)
	}

	views, err := buildLoadedModelViews(context.Background(), db)
	if err != nil {
		t.Fatalf("failed to build loaded model views: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("expected 2 views, got %+v", views)
	}

	var shared, other *LoadedModelView
	for i := range views {
		if string(views[i].Model.ModelCacheHash) == string(taskA.RequestedModelCacheHash) {
			shared = &views[i]
		} else {
			other = &views[i]
		}
	}
	if shared == nil || other == nil {
		t.Fatalf("missing views: %+v", views)
	}
	if shared.Model.MinVRAM != 16 || shared.OnDiskNodeCount != 3 || shared.InMemoryNodeCount != 2 {
		t.Fatalf("unexpected shared view: min_vram=%d disk=%d memory=%d", shared.Model.MinVRAM, shared.OnDiskNodeCount, shared.InMemoryNodeCount)
	}
	if other.Model.MinVRAM != 32 || other.OnDiskNodeCount != 2 || other.InMemoryNodeCount != 0 {
		t.Fatalf("unexpected other view: min_vram=%d disk=%d memory=%d", other.Model.MinVRAM, other.OnDiskNodeCount, other.InMemoryNodeCount)
	}
}
