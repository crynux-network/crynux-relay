package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"sync"
	"testing"
)

func resetNodeIndexForTest() {
	globalNodeIndex.mu.Lock()
	defer globalNodeIndex.mu.Unlock()
	globalNodeIndex.slots = make(map[string]*nodeIndexSlot)
}

func findIndexEntry(t *testing.T, address string) *NodeIndexEntry {
	t.Helper()
	for _, entry := range SnapshotNodeIndex() {
		if entry.Address == address {
			return entry
		}
	}
	return nil
}

func setupNodeIndexTestDB(t *testing.T) {
	t.Helper()
	initServiceTestConfig(t)
	resetNodeIndexForTest()
	if err := config.GetDB().AutoMigrate(&models.Node{}, &models.NodeModel{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
}

func TestInitNodeIndexRebuildsNonQuitNodes(t *testing.T) {
	setupNodeIndexTestDB(t)
	db := config.GetDB()
	ctx := context.Background()

	nodes := []models.Node{
		{Address: "0xavailable", Status: models.NodeStatusAvailable, GPUName: "RTX 4090", GPUVram: 24, StakeAmount: models.BigInt{Int: *big.NewInt(100)}},
		{Address: "0xbusy", Status: models.NodeStatusBusy, GPUName: "A100", GPUVram: 40, StakeAmount: models.BigInt{Int: *big.NewInt(200)}},
		{Address: "0xquit", Status: models.NodeStatusQuit, GPUName: "L40", GPUVram: 24, StakeAmount: models.BigInt{Int: *big.NewInt(0)}},
	}
	for i := range nodes {
		if err := db.Create(&nodes[i]).Error; err != nil {
			t.Fatalf("failed to create node: %v", err)
		}
	}
	if err := db.Create(&models.NodeModel{NodeAddress: "0xavailable", ModelID: "base:model-a", InUse: true}).Error; err != nil {
		t.Fatalf("failed to create node model: %v", err)
	}

	if err := InitNodeIndex(ctx, db); err != nil {
		t.Fatalf("failed to init node index: %v", err)
	}

	entries := SnapshotNodeIndex()
	if len(entries) != 2 {
		t.Fatalf("expected 2 index entries, got %d", len(entries))
	}
	if findIndexEntry(t, "0xquit") != nil {
		t.Fatal("expected quit node to be excluded from the index")
	}

	entry := findIndexEntry(t, "0xavailable")
	if entry == nil {
		t.Fatal("expected available node in the index")
	}
	if _, ok := entry.OnDiskModelIDs["base:model-a"]; !ok {
		t.Fatal("expected on-disk model set to contain base:model-a")
	}
	if _, ok := entry.InUseModelIDs["base:model-a"]; !ok {
		t.Fatal("expected in-use model set to contain base:model-a")
	}
}

func TestExecuteNodeStateUpdateRefreshesIndexFromCommittedState(t *testing.T) {
	setupNodeIndexTestDB(t)
	db := config.GetDB()
	ctx := context.Background()

	node := models.Node{Address: "0xnode", Status: models.NodeStatusAvailable, StakeAmount: models.BigInt{Int: *big.NewInt(100)}}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := InitNodeIndex(ctx, db); err != nil {
		t.Fatalf("failed to init node index: %v", err)
	}

	if err := ExecuteNodeStateUpdate(ctx, db, []string{"0xnode"}, func() error {
		return db.Model(&models.Node{}).Where("address = ?", "0xnode").Update("status", models.NodeStatusPaused).Error
	}); err != nil {
		t.Fatalf("execute node state update: %v", err)
	}

	entry := findIndexEntry(t, "0xnode")
	if entry == nil {
		t.Fatal("expected node in the index")
	}
	if entry.Status != models.NodeStatusPaused {
		t.Fatalf("expected index status paused, got %d", entry.Status)
	}
}

func TestExecuteNodeStateUpdateRemovesQuitNode(t *testing.T) {
	setupNodeIndexTestDB(t)
	db := config.GetDB()
	ctx := context.Background()

	node := models.Node{Address: "0xnode", Status: models.NodeStatusAvailable, StakeAmount: models.BigInt{Int: *big.NewInt(100)}}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := InitNodeIndex(ctx, db); err != nil {
		t.Fatalf("failed to init node index: %v", err)
	}

	if err := ExecuteNodeStateUpdate(ctx, db, []string{"0xnode"}, func() error {
		return db.Model(&models.Node{}).Where("address = ?", "0xnode").Update("status", models.NodeStatusQuit).Error
	}); err != nil {
		t.Fatalf("execute node state update: %v", err)
	}

	if findIndexEntry(t, "0xnode") != nil {
		t.Fatal("expected quit node to be removed from the index")
	}
}

func TestExecuteNodeStateUpdateRefreshesEvenWhenFnFails(t *testing.T) {
	setupNodeIndexTestDB(t)
	db := config.GetDB()
	ctx := context.Background()

	node := models.Node{Address: "0xnode", Status: models.NodeStatusAvailable, StakeAmount: models.BigInt{Int: *big.NewInt(100)}}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := InitNodeIndex(ctx, db); err != nil {
		t.Fatalf("failed to init node index: %v", err)
	}

	failure := context.DeadlineExceeded
	err := ExecuteNodeStateUpdate(ctx, db, []string{"0xnode"}, func() error {
		if err := db.Model(&models.Node{}).Where("address = ?", "0xnode").Update("status", models.NodeStatusBusy).Error; err != nil {
			return err
		}
		return failure
	})
	if err != failure {
		t.Fatalf("expected fn error to propagate, got %v", err)
	}

	entry := findIndexEntry(t, "0xnode")
	if entry == nil {
		t.Fatal("expected node in the index")
	}
	if entry.Status != models.NodeStatusBusy {
		t.Fatalf("expected index refreshed to committed busy state, got %d", entry.Status)
	}
}

func TestExecuteNodeStateUpdateSerializesConcurrentUpdates(t *testing.T) {
	setupNodeIndexTestDB(t)
	db := config.GetDB()
	ctx := context.Background()

	node := models.Node{Address: "0xnode", Status: models.NodeStatusAvailable, StakeAmount: models.BigInt{Int: *big.NewInt(0)}}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := InitNodeIndex(ctx, db); err != nil {
		t.Fatalf("failed to init node index: %v", err)
	}

	inCritical := 0
	maxInCritical := 0
	var observed sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ExecuteNodeStateUpdate(ctx, db, []string{"0xnode"}, func() error {
				observed.Lock()
				inCritical++
				if inCritical > maxInCritical {
					maxInCritical = inCritical
				}
				observed.Unlock()

				err := db.Model(&models.Node{}).Where("address = ?", "0xnode").
					Update("stake_amount", models.BigInt{Int: *big.NewInt(1)}).Error

				observed.Lock()
				inCritical--
				observed.Unlock()
				return err
			})
		}()
	}
	wg.Wait()

	if maxInCritical != 1 {
		t.Fatalf("expected per-node critical sections to be serialized, got concurrency %d", maxInCritical)
	}
}
