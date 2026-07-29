package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newNodeCapabilitiesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Node{},
		&models.NodeModel{},
		&models.NodeNameCount{},
		&models.NetworkNodeData{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func seedNodeCapabilitiesTestNode(t *testing.T, db *gorm.DB, status models.NodeStatus) models.Node {
	t.Helper()
	node := models.Node{
		Address:      "0xnode",
		Network:      "base",
		Status:       status,
		GPUName:      "RTX 3090+docker",
		GPUVram:      24,
		MajorVersion: 1,
		MinorVersion: 0,
		PatchVersion: 0,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	modelsToCreate := []models.NodeModel{
		models.NewNodeModel(node.Address, "base:model-retained", true),
		models.NewNodeModel(node.Address, "base:model-removed", false),
	}
	if err := db.Create(&modelsToCreate).Error; err != nil {
		t.Fatalf("failed to create node models: %v", err)
	}
	networkData := models.NetworkNodeData{
		Address:   node.Address,
		Network:   node.Network,
		CardModel: node.GPUName,
		VRam:      int(node.GPUVram),
	}
	if err := db.Create(&networkData).Error; err != nil {
		t.Fatalf("failed to create network node data: %v", err)
	}
	if IsNodeStatusActiveForNodeNameCount(status) {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return IncrementNodeNameCountTx(context.Background(), tx, &node)
		}); err != nil {
			t.Fatalf("failed to seed node name count: %v", err)
		}
	}
	if err := InitNodeIndex(context.Background(), db); err != nil {
		t.Fatalf("failed to initialize node index: %v", err)
	}
	return node
}

func TestSyncNodeCapabilitiesReplacesInventoryAndRefreshesMatchingState(t *testing.T) {
	resetNodeNamePolicyCacheForTest()
	ctx := context.Background()
	db := newNodeCapabilitiesTestDB(t)
	oldNode := seedNodeCapabilitiesTestNode(t, db, models.NodeStatusAvailable)
	if err := RefreshNodeNameCountCache(ctx, db); err != nil {
		t.Fatalf("failed to load node name count cache: %v", err)
	}

	err := SyncNodeCapabilities(ctx, db, oldNode.Address, NodeCapabilities{
		GPUName:      "  RTX   4090+docker ",
		GPUVram:      48,
		MajorVersion: 2,
		MinorVersion: 1,
		PatchVersion: 3,
		ModelIDs:     []string{"BASE:MODEL-RETAINED", "base:model-added", "BASE:MODEL-ADDED"},
	})
	if err != nil {
		t.Fatalf("sync capabilities failed: %v", err)
	}

	node, err := models.GetNodeWithModelsByAddress(ctx, db, oldNode.Address)
	if err != nil {
		t.Fatalf("failed to load updated node: %v", err)
	}
	if node.GPUName != "RTX 4090+docker" || node.GPUVram != 48 ||
		node.MajorVersion != 2 || node.MinorVersion != 1 || node.PatchVersion != 3 {
		t.Fatalf("unexpected updated node: %#v", node)
	}
	if len(node.Models) != 2 {
		t.Fatalf("unexpected model count: %d", len(node.Models))
	}
	modelState := make(map[string]bool, len(node.Models))
	for _, model := range node.Models {
		modelState[model.ModelID] = model.InUse
	}
	if !modelState["base:model-retained"] {
		t.Fatal("retained model should preserve in_use")
	}
	if modelState["base:model-added"] {
		t.Fatal("new model should not be in use")
	}
	if _, ok := modelState["base:model-removed"]; ok {
		t.Fatal("absent model should be removed")
	}

	oldCount, err := GetNodeNameActiveCount(ctx, db, oldNode.GPUName, oldNode.GPUVram, "1.0.0")
	if err != nil {
		t.Fatalf("failed to read old node name count: %v", err)
	}
	newCount, err := GetNodeNameActiveCount(ctx, db, node.GPUName, node.GPUVram, "2.1.3")
	if err != nil {
		t.Fatalf("failed to read new node name count: %v", err)
	}
	if oldCount != 0 || newCount != 1 {
		t.Fatalf("unexpected node name counts: old=%d new=%d", oldCount, newCount)
	}

	entries := SnapshotNodeIndex()
	if len(entries) != 1 {
		t.Fatalf("unexpected node index size: %d", len(entries))
	}
	entry := entries[0]
	if entry.GPUName != node.GPUName || entry.GPUVram != node.GPUVram || entry.MajorVersion != 2 {
		t.Fatalf("unexpected node index capabilities: %#v", entry)
	}
	if _, ok := entry.OnDiskModelIDs["base:model-added"]; !ok {
		t.Fatal("node index should contain added model")
	}
	if _, ok := entry.InUseModelIDs["base:model-retained"]; !ok {
		t.Fatal("node index should preserve retained in-use model")
	}

	var networkData models.NetworkNodeData
	if err := db.Where("address = ?", oldNode.Address).First(&networkData).Error; err != nil {
		t.Fatalf("failed to load network node data: %v", err)
	}
	if networkData.CardModel != node.GPUName || networkData.VRam != int(node.GPUVram) {
		t.Fatalf("unexpected network node data: %#v", networkData)
	}
}

func TestSyncNodeCapabilitiesDoesNotTransferPausedNodeNameCount(t *testing.T) {
	resetNodeNamePolicyCacheForTest()
	ctx := context.Background()
	db := newNodeCapabilitiesTestDB(t)
	node := seedNodeCapabilitiesTestNode(t, db, models.NodeStatusPaused)

	if err := SyncNodeCapabilities(ctx, db, node.Address, NodeCapabilities{
		GPUName:      "RTX 4090+docker",
		GPUVram:      48,
		MajorVersion: 2,
		MinorVersion: 0,
		PatchVersion: 0,
		ModelIDs:     []string{"base:model-retained"},
	}); err != nil {
		t.Fatalf("sync paused node capabilities failed: %v", err)
	}

	var count int64
	if err := db.Model(&models.NodeNameCount{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count node name rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("paused capability sync should not create node name counts, got %d", count)
	}
}

func TestSyncNodeCapabilitiesRejectsQuitNode(t *testing.T) {
	db := newNodeCapabilitiesTestDB(t)
	node := seedNodeCapabilitiesTestNode(t, db, models.NodeStatusQuit)

	err := SyncNodeCapabilities(context.Background(), db, node.Address, NodeCapabilities{
		GPUName:      node.GPUName,
		GPUVram:      node.GPUVram,
		MajorVersion: node.MajorVersion,
		MinorVersion: node.MinorVersion,
		PatchVersion: node.PatchVersion,
	})
	if !errors.Is(err, ErrNodeCapabilitiesIllegalStatus) {
		t.Fatalf("expected illegal status error, got %v", err)
	}
}
