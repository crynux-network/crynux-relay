package service

import (
	"bytes"
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"testing"
	"time"

	"gorm.io/gorm"
)

func setRequiredModelsByVRAMForTest(t *testing.T, items []config.RequiredModelsByVRAMConfig) {
	t.Helper()
	cfg := config.GetConfig()
	original := cfg.ModelDistribution.RequiredModelsByVRAM
	cfg.ModelDistribution.RequiredModelsByVRAM = items
	t.Cleanup(func() {
		cfg.ModelDistribution.RequiredModelsByVRAM = original
	})
}

func requiredLLMConfigItem(minVRAM uint64, repository, revision string) config.RequiredModelsByVRAMConfig {
	return config.RequiredModelsByVRAMConfig{
		MinVRAM:  minVRAM,
		TaskType: uint8(models.TaskTypeLLM),
		ModelFiles: []config.RequiredModelFileEntryConfig{{
			SourceType: models.ModelSourceTypeHuggingFace,
			Repository: repository,
			Revision:   revision,
		}},
	}
}

func TestEnsureRequiredModelDownloadsMatchesMinVRAM(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()

	setRequiredModelsByVRAMForTest(t, []config.RequiredModelsByVRAMConfig{
		requiredLLMConfigItem(24, "org/model-24", "rev-24"),
		requiredLLMConfigItem(48, "org/model-48", "rev-48"),
	})

	low := newDistributionTestNode("0xlow")
	low.GPUVram = 16
	mid := newDistributionTestNode("0xmid")
	mid.GPUVram = 24
	high := newDistributionTestNode("0xhigh")
	high.GPUVram = 48
	for _, node := range []models.Node{low, mid, high} {
		if err := db.Create(&node).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
	}

	if err := EnsureRequiredModelDownloads(ctx, db, &low); err != nil {
		t.Fatalf("ensure low: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 0 {
		t.Fatalf("expected no download for VRAM below min, got %d", count)
	}

	if err := EnsureRequiredModelDownloads(ctx, db, &mid); err != nil {
		t.Fatalf("ensure mid: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected one download for 24GB node, got %d", count)
	}

	if err := EnsureRequiredModelDownloads(ctx, db, &high); err != nil {
		t.Fatalf("ensure high: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 3 {
		t.Fatalf("expected three downloads after 48GB node (two tiers), got %d", count)
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 3 {
		t.Fatalf("expected three selections, got %d", len(selections))
	}
}

func TestEnsureRequiredModelDownloadsIdempotent(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()

	setRequiredModelsByVRAMForTest(t, []config.RequiredModelsByVRAMConfig{
		requiredLLMConfigItem(24, "org/model-idem", "rev-1"),
	})

	node := newDistributionTestNode("0xidem")
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	if err := EnsureRequiredModelDownloads(ctx, db, &node); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if err := EnsureRequiredModelDownloads(ctx, db, &node); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected one download with pending selection, got %d", count)
	}

	seedNodeModelFiles(t, db, node.Address, testModelFile(t, "org/model-idem", "rev-1"))
	if err := EnsureRequiredModelDownloads(ctx, db, &node); err != nil {
		t.Fatalf("ensure after inventory: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected no download when inventory covers the set, got %d", count)
	}
}

func TestCleanupSelectionsKeepsRequiredPending(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	exact := []models.Hash32{testModelFileHash(t, "org/required", "rev-1")}
	setRequiredModelsByVRAMForTest(t, []config.RequiredModelsByVRAMConfig{
		requiredLLMConfigItem(24, "org/required", "rev-1"),
	})

	pending := models.NewNodeModelDownloadSelection(exact, nil, "0xpending", 24, now, now.Add(time.Hour))
	completed := models.NewNodeModelDownloadSelection(exact, nil, "0xdone", 24, now.Add(-time.Hour), now.Add(time.Hour))
	completed.Status = models.NodeModelDownloadSelectionCompleted
	staleExact := []models.Hash32{testModelFileHash(t, "org/stale", "rev-1")}
	stale := models.NewNodeModelDownloadSelection(staleExact, nil, "0xstale", 24, now, now.Add(time.Hour))
	for _, selection := range []*models.NodeModelDownloadSelection{pending, completed, stale} {
		if err := models.CreateNodeModelDownloadSelection(ctx, db, selection); err != nil {
			t.Fatalf("create selection: %v", err)
		}
	}

	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("round: %v", err)
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 1 {
		t.Fatalf("expected only the required pending selection to remain, got %d", len(selections))
	}
	if selections[0].NodeAddress != "0xpending" ||
		!bytes.Equal(selections[0].ModelFileHash, models.DistributionKey(exact, nil).Bytes()) {
		t.Fatalf("unexpected remaining selection: %+v", selections[0])
	}
}

func TestSyncNodeCapabilitiesEmitsRequiredModelDownload(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	resetNodeNamePolicyCacheForTest()

	setRequiredModelsByVRAMForTest(t, []config.RequiredModelsByVRAMConfig{
		requiredLLMConfigItem(24, "org/caps-model", "rev-1"),
	})

	if err := db.AutoMigrate(
		&models.NodeNameCount{},
		&models.NetworkNodeData{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	node := newDistributionTestNode("0xcaps")
	node.MajorVersion = 1
	node.MinorVersion = 0
	node.PatchVersion = 0
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return IncrementNodeNameCountTx(ctx, tx, &node)
	}); err != nil {
		t.Fatalf("seed node name count: %v", err)
	}
	networkData := models.NetworkNodeData{
		Address:   node.Address,
		Network:   "test",
		CardModel: node.GPUName,
		VRam:      int(node.GPUVram),
	}
	if err := db.Create(&networkData).Error; err != nil {
		t.Fatalf("create network data: %v", err)
	}
	if err := InitNodeIndex(ctx, db); err != nil {
		t.Fatalf("init node index: %v", err)
	}
	if err := RefreshNodeNameCountCache(ctx, db); err != nil {
		t.Fatalf("refresh node name count cache: %v", err)
	}

	if err := SyncNodeCapabilities(ctx, db, node.Address, NodeCapabilities{
		GPUName:      node.GPUName,
		GPUVram:      node.GPUVram,
		MajorVersion: node.MajorVersion,
		MinorVersion: node.MinorVersion,
		PatchVersion: node.PatchVersion,
		ModelFiles:   nil,
	}); err != nil {
		t.Fatalf("sync capabilities: %v", err)
	}

	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected one DownloadModel after capabilities sync, got %d", count)
	}
	selections, err := models.GetNodeModelDownloadSelectionsByNodeAddress(ctx, db, node.Address)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 1 || selections[0].Status != models.NodeModelDownloadSelectionPending {
		t.Fatalf("unexpected selections: %+v", selections)
	}
}
