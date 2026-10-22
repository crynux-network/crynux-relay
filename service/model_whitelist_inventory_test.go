package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"testing"

	"gorm.io/gorm"
)

func setupModelWhitelistInventoryTest(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	resetModelWhitelistCacheForTest()
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.ModelWhitelist{}, &models.Node{}, &models.LoadedModel{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	migrateModelStateTables(t, db)
	ctx := context.Background()
	if err := InitModelWhitelistCache(ctx, db); err != nil {
		t.Fatalf("init whitelist cache: %v", err)
	}
	return db, ctx
}

func enableModelWhitelist(t *testing.T) {
	t.Helper()
	config.GetConfig().Task.ModelWhitelistEnabled = true
	t.Cleanup(func() { config.GetConfig().Task.ModelWhitelistEnabled = false })
}

func TestFilterWhitelistedModelFilesNormalizesCase(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}

	files := []models.ModelFile{
		testModelFile(t, "Qwen/Foo", "rev-1"),
		testModelFile(t, "evil/pearl", "rev-1"),
	}
	kept, err := filterWhitelistedModelFiles(files)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(kept) != 1 {
		t.Fatalf("expected 1 kept file, got %d", len(kept))
	}
	entry, err := kept[0].Entry()
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	if entry.Repository != "qwen/foo" {
		t.Fatalf("unexpected repository: %s", entry.Repository)
	}
}

func TestAddNodeModelFileSkipsNonWhitelisted(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	node := models.Node{Address: "0xwl-add", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	allowed := testModelFile(t, "Qwen/Foo", "rev-1")
	blocked := testModelFile(t, "evil/pearl", "rev-1")
	if err := AddNodeModelFile(ctx, db, node.Address, blocked); err != nil {
		t.Fatalf("blocked add should no-op: %v", err)
	}
	if err := AddNodeModelFile(ctx, db, node.Address, allowed); err != nil {
		t.Fatalf("allowed add: %v", err)
	}

	rows, err := models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil {
		t.Fatalf("list inventory: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one inventory row, got %d", len(rows))
	}
	var catalogCount int64
	if err := db.Model(&models.ModelFile{}).Count(&catalogCount).Error; err != nil {
		t.Fatalf("count catalog: %v", err)
	}
	if catalogCount != 1 {
		t.Fatalf("expected only allowed catalog row, got %d", catalogCount)
	}
}

func TestReplaceNodeModelFilesDropsNonWhitelisted(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	node := models.Node{Address: "0xwl-replace", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	pearl := testModelFile(t, "evil/pearl", "rev-1")
	seedNodeModelFiles(t, db, node.Address, pearl)

	allowed := testModelFile(t, "qwen/foo", "rev-1")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return replaceNodeModelFiles(ctx, tx, node.Address, []models.ModelFile{allowed, pearl})
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	rows, err := models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil {
		t.Fatalf("list inventory: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected only whitelisted inventory, got %d", len(rows))
	}
	wantHash := testModelFileHash(t, "qwen/foo", "rev-1")
	gotHash, err := models.Hash32FromBytes(rows[0].ModelFileHash)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if gotHash != wantHash {
		t.Fatalf("unexpected retained hash")
	}
}

func TestSetNodeInMemoryModelRejectsNonWhitelisted(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	node := models.Node{Address: "0xwl-memory", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	err := SetNodeInMemoryModel(ctx, db, node.Address, &NodeInMemoryModelReport{
		CacheHash:      models.Hash32{1},
		Files:          []models.ModelFile{testModelFile(t, "evil/pearl", "rev-1")},
		CacheConfig:    []byte(`{}`),
		ExecutionDType: "float16",
	})
	if !errors.Is(err, ErrModelNotWhitelisted) {
		t.Fatalf("expected ErrModelNotWhitelisted, got %v", err)
	}
}

func TestUpdateLoadedModelsSkipsNonWhitelisted(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	loadedModelCache = newLoadedModelMinVRAMCache()

	updateLoadedModels(newLoadedModelTask(t, llmLoadedModelArgs("evil/pearl", `null`), models.TaskTypeLLM, "evil/pearl", "float16", testModelFileHash(t, "evil/pearl", "rev-1")), &models.Node{GPUVram: 16})
	updateLoadedModels(newLoadedModelTask(t, llmLoadedModelArgs("qwen/foo", `null`), models.TaskTypeLLM, "qwen/foo", "float16", testModelFileHash(t, "qwen/foo", "rev-1")), &models.Node{GPUVram: 16})
	flushLoadedModelCache(ctx, db)

	var rows []models.LoadedModel
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("query loaded models: %v", err)
	}
	if len(rows) != 1 || rows[0].ModelID != "qwen/foo" {
		t.Fatalf("unexpected loaded models: %+v", rows)
	}
}

func TestGetLoadedModelViewsOmitsNonWhitelisted(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}

	for _, repository := range []string{"evil/pearl", "qwen/foo"} {
		task := newLoadedModelTask(t, llmLoadedModelArgs(repository, `null`), models.TaskTypeLLM, repository, "float16", testModelFileHash(t, repository, "rev-1"))
		row := newLoadedModelRow(t, task, 16)
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create %s loaded model: %v", repository, err)
		}
	}

	invalidateLoadedModelViewCache()
	views, err := GetLoadedModelViews(ctx, db)
	if err != nil {
		t.Fatalf("get views: %v", err)
	}
	if len(views) != 1 || views[0].Model.ModelID != "qwen/foo" {
		t.Fatalf("unexpected views: %+v", views)
	}
}

func TestPruneNonWhitelistedInventoryOnRefresh(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	node := models.Node{Address: "0xwl-prune", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	pearl := testModelFile(t, "evil/pearl", "rev-1")
	foo := testModelFile(t, "qwen/foo", "rev-1")
	seedNodeModelFiles(t, db, node.Address, pearl, foo)
	memory := models.NewNodeInMemoryModel(node.Address, models.Hash32{9}, []models.Hash32{testModelFileHash(t, "evil/pearl", "rev-1")}, []byte(`{}`), "float16")
	if err := db.Create(&memory).Error; err != nil {
		t.Fatalf("create in-memory: %v", err)
	}
	if err := db.Create(&models.LoadedModel{
		RequiredModelFilesHash:  models.RequiredModelFilesHash([]models.Hash32{testModelFileHash(t, "evil/pearl", "rev-1")}).Bytes(),
		ModelCacheHash:          models.Hash32{1}.Bytes(),
		ExecutionDType:          "float16",
		RequiredModelFileHashes: models.EncodeHashList([]models.Hash32{testModelFileHash(t, "evil/pearl", "rev-1")}),
		QuantizationConfig:      models.JSONText(`null`),
		ModelCacheConfig:        models.JSONText(`{}`),
		ModelID:                 "evil/pearl",
		ModelType:               models.LoadedModelTypeLLM,
		MinVRAM:                 16,
	}).Error; err != nil {
		t.Fatalf("create loaded model: %v", err)
	}

	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}

	rows, err := models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil {
		t.Fatalf("list inventory: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected pruned inventory size 1, got %d", len(rows))
	}
	storedMemory, err := models.GetNodeInMemoryModel(ctx, db, node.Address)
	if err != nil {
		t.Fatalf("get in-memory: %v", err)
	}
	if storedMemory != nil {
		t.Fatalf("expected in-memory cleared, got %+v", storedMemory)
	}
	var loadedCount int64
	if err := db.Model(&models.LoadedModel{}).Count(&loadedCount).Error; err != nil {
		t.Fatalf("count loaded models: %v", err)
	}
	if loadedCount != 0 {
		t.Fatalf("expected loaded models pruned, got %d", loadedCount)
	}
}

func TestWhitelistAddDoesNotRestoreInventoryUntilSync(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	enableModelWhitelist(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/foo"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	node := models.Node{Address: "0xwl-restore", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	pending := testModelFile(t, "qwen/bar", "rev-1")
	if err := AddNodeModelFile(ctx, db, node.Address, pending); err != nil {
		t.Fatalf("add pending model: %v", err)
	}
	rows, err := models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil || len(rows) != 0 {
		t.Fatalf("expected empty inventory before whitelist add, got %v, %v", rows, err)
	}

	if err := AddModelWhitelistName(ctx, db, "qwen/bar"); err != nil {
		t.Fatalf("add bar to whitelist: %v", err)
	}
	rows, err = models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil || len(rows) != 0 {
		t.Fatalf("expected inventory still empty after whitelist add, got %v, %v", rows, err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return replaceNodeModelFiles(ctx, tx, node.Address, []models.ModelFile{pending})
	}); err != nil {
		t.Fatalf("sync replace: %v", err)
	}
	rows, err = models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil || len(rows) != 1 {
		t.Fatalf("expected inventory after sync, got %v, %v", rows, err)
	}
}

func TestModelWhitelistDisabledSkipsInventoryFilter(t *testing.T) {
	db, ctx := setupModelWhitelistInventoryTest(t)
	config.GetConfig().Task.ModelWhitelistEnabled = false
	node := models.Node{Address: "0xwl-disabled", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	file := testModelFile(t, "evil/pearl", "rev-1")
	if err := AddNodeModelFile(ctx, db, node.Address, file); err != nil {
		t.Fatalf("add: %v", err)
	}
	rows, err := models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil || len(rows) != 1 {
		t.Fatalf("expected inventory retained when disabled, got %v, %v", rows, err)
	}
}
