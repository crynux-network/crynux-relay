package models

import (
	"context"
	dbmodels "crynux_relay/models"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetLoadedModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	originalGetDB := getDB
	getDB = func() *gorm.DB {
		return db
	}
	defer func() {
		getDB = originalGetDB
	}()

	if err := db.AutoMigrate(&dbmodels.LoadedModel{}, &dbmodels.ModelFile{}, &dbmodels.NodeModel{}, &dbmodels.NodeInMemoryModel{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	fileA := newTestModelFile(t, "a/model", "rev-1")
	fileZ := newTestModelFile(t, "z/model", "rev-1")
	hashA := hashOf(t, fileA)
	hashZ := hashOf(t, fileZ)
	if err := dbmodels.UpsertModelFiles(context.Background(), db, []dbmodels.ModelFile{fileA, fileZ}); err != nil {
		t.Fatalf("failed to create model files: %v", err)
	}

	loadedModels := []dbmodels.LoadedModel{
		newTestLoadedModel("z/model", dbmodels.LoadedModelTypeSD, 24, "float16", []byte(`{"task_kind":"image"}`), hashZ),
		newTestLoadedModel("a/model", dbmodels.LoadedModelTypeLLM, 16, "bfloat16", []byte(`{"task_kind":"llm"}`), hashA),
		newTestLoadedModel("b/model", dbmodels.LoadedModelTypeLLM, 8, "float16", nil, hashA),
	}
	cacheHash := sha256.Sum256([]byte(`{"task_kind":"llm"}`))
	if err := db.Create(&loadedModels).Error; err != nil {
		t.Fatalf("failed to create loaded models: %v", err)
	}
	nodeModels := []dbmodels.NodeModel{
		dbmodels.NewNodeModel("0x1", hashA),
		dbmodels.NewNodeModel("0x2", hashA),
		dbmodels.NewNodeModel("0x3", hashZ),
	}
	if err := db.Create(&nodeModels).Error; err != nil {
		t.Fatalf("failed to create node models: %v", err)
	}
	inMemory := []dbmodels.NodeInMemoryModel{
		dbmodels.NewNodeInMemoryModel("0x1", dbmodels.Hash32(cacheHash), []dbmodels.Hash32{hashA}, []byte(`{}`), "bfloat16"),
		dbmodels.NewNodeInMemoryModel("0x2", dbmodels.Hash32(cacheHash), []dbmodels.Hash32{hashA}, []byte(`{}`), "float16"),
	}
	if err := db.Create(&inMemory).Error; err != nil {
		t.Fatalf("failed to create in-memory models: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v2/loaded-models", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	resp, err := GetLoadedModels(c)
	if err != nil {
		t.Fatalf("GetLoadedModels failed: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 loaded models, got %d", len(resp.Data))
	}
	wantHash := dbmodels.Hash32(cacheHash).Hex()
	if resp.Data[0].ModelCacheHash != wantHash || string(resp.Data[0].ModelCacheConfig) != `{"task_kind":"llm"}` {
		t.Fatalf("unexpected first loaded model cache identity: %+v", resp.Data[0])
	}
	if resp.Data[0].ModelID != "a/model" || resp.Data[0].ModelType != dbmodels.LoadedModelTypeLLM || resp.Data[0].MinVRAM != 16 || resp.Data[0].ExecutionDType != "bfloat16" {
		t.Fatalf("unexpected first loaded model: %+v", resp.Data[0])
	}
	if resp.Data[0].OnDiskNodeCount != 2 || resp.Data[0].InMemoryNodeCount != 1 {
		t.Fatalf("unexpected first loaded model node counts: %+v", resp.Data[0])
	}
	if resp.Data[1].ModelID != "z/model" || resp.Data[1].ModelType != dbmodels.LoadedModelTypeSD || resp.Data[1].MinVRAM != 24 {
		t.Fatalf("unexpected second loaded model: %+v", resp.Data[1])
	}
	if resp.Data[1].OnDiskNodeCount != 1 || resp.Data[1].InMemoryNodeCount != 0 {
		t.Fatalf("unexpected second loaded model node counts: %+v", resp.Data[1])
	}
}

func newTestModelFile(t *testing.T, repository, revision string) dbmodels.ModelFile {
	t.Helper()
	file, err := dbmodels.NewModelFile(dbmodels.ModelFileEntry{
		SourceType:   dbmodels.ModelSourceTypeHuggingFace,
		Repository:   repository,
		Revision:     revision,
		FormatKwargs: map[string]any{},
	})
	if err != nil {
		t.Fatalf("failed to build model file: %v", err)
	}
	return *file
}

func hashOf(t *testing.T, file dbmodels.ModelFile) dbmodels.Hash32 {
	t.Helper()
	hash, err := dbmodels.Hash32FromBytes(file.ModelFileHash)
	if err != nil {
		t.Fatalf("invalid model file hash: %v", err)
	}
	return hash
}

// newTestLoadedModel builds a loaded model whose cache hash is the hash of
// cacheConfig. A nil cacheConfig produces an incomplete row.
func newTestLoadedModel(modelID string, modelType dbmodels.LoadedModelType, minVRAM uint64, dtype string, cacheConfig []byte, fileHash dbmodels.Hash32) dbmodels.LoadedModel {
	cacheHash := sha256.Sum256(cacheConfig)
	if cacheConfig == nil {
		cacheConfig = []byte(`not json`)
	}
	return dbmodels.LoadedModel{
		RequiredModelFilesHash:  dbmodels.RequiredModelFilesHash([]dbmodels.Hash32{fileHash}).Bytes(),
		ModelCacheHash:          cacheHash[:],
		ExecutionDType:          dtype,
		RequiredModelFileHashes: dbmodels.EncodeHashList([]dbmodels.Hash32{fileHash}),
		QuantizationConfig:      dbmodels.JSONText(`null`),
		ModelID:                 modelID,
		ModelCacheConfig:        cacheConfig,
		ModelType:               modelType,
		MinVRAM:                 minVRAM,
	}
}
