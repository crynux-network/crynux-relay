package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newNodeNamePolicyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeNameWhitelist{}, &models.NodeNameCount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestNodeNameWhitelistCRUDAndCache(t *testing.T) {
	resetNodeNamePolicyCacheForTest()

	ctx := context.Background()
	db := newNodeNamePolicyTestDB(t)
	gpuName := "NVIDIA RTX 4090"
	gpuVram := uint64(24)
	nodeVersion := "1.2.3"

	if err := AddNodeNameWhitelist(ctx, db, gpuName, gpuVram, nodeVersion); err != nil {
		t.Fatalf("add whitelist should succeed: %v", err)
	}
	if err := AddNodeNameWhitelist(ctx, db, gpuName, gpuVram, nodeVersion); !errors.Is(err, ErrNodeNameWhitelistExists) {
		t.Fatalf("expected ErrNodeNameWhitelistExists, got %v", err)
	}

	entries, err := ListNodeNameWhitelist(ctx, db)
	if err != nil {
		t.Fatalf("list whitelist should succeed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("unexpected entry count: %d", len(entries))
	}
	if entries[0].GPUName != gpuName || entries[0].GPUVram != gpuVram || entries[0].NodeVersion != nodeVersion {
		t.Fatalf("unexpected whitelist entry: %#v", entries[0])
	}

	allowed, err := IsNodeNameWhitelisted(ctx, db, gpuName, gpuVram, nodeVersion)
	if err != nil {
		t.Fatalf("whitelist check should succeed: %v", err)
	}
	if !allowed {
		t.Fatal("entry should be allowed")
	}

	if err := DeleteNodeNameWhitelist(ctx, db, gpuName, gpuVram, nodeVersion); err != nil {
		t.Fatalf("delete whitelist should succeed: %v", err)
	}
	allowed, err = IsNodeNameWhitelisted(ctx, db, gpuName, gpuVram, nodeVersion)
	if err != nil {
		t.Fatalf("whitelist check after delete should succeed: %v", err)
	}
	if allowed {
		t.Fatal("entry should be disallowed after delete")
	}
	if err := DeleteNodeNameWhitelist(ctx, db, gpuName, gpuVram, nodeVersion); !errors.Is(err, ErrNodeNameWhitelistMissing) {
		t.Fatalf("expected ErrNodeNameWhitelistMissing, got %v", err)
	}
}

func TestNodeNameCountTxAndCache(t *testing.T) {
	resetNodeNamePolicyCacheForTest()

	ctx := context.Background()
	db := newNodeNamePolicyTestDB(t)
	node := &models.Node{
		GPUName:      "NVIDIA RTX 3090",
		GPUVram:      24,
		MajorVersion: 1,
		MinorVersion: 0,
		PatchVersion: 5,
	}
	nodeVersion := BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion)

	if err := db.Transaction(func(tx *gorm.DB) error {
		return IncrementNodeNameCountTx(ctx, tx, node)
	}); err != nil {
		t.Fatalf("increment tx should succeed: %v", err)
	}
	if _, err := GetNodeNameActiveCount(ctx, db, node.GPUName, node.GPUVram, nodeVersion); err != nil {
		t.Fatalf("get active count should succeed: %v", err)
	}
	ApplyNodeNameCountDeltaToCache(node.GPUName, node.GPUVram, nodeVersion, 1)

	count, err := GetNodeNameActiveCount(ctx, db, node.GPUName, node.GPUVram, nodeVersion)
	if err != nil {
		t.Fatalf("get active count should succeed: %v", err)
	}
	if count != 2 {
		t.Fatalf("unexpected active count after cache delta: %d", count)
	}

	if err := RefreshNodeNameCountCache(ctx, db); err != nil {
		t.Fatalf("refresh count cache should succeed: %v", err)
	}
	count, err = GetNodeNameActiveCount(ctx, db, node.GPUName, node.GPUVram, nodeVersion)
	if err != nil {
		t.Fatalf("get active count after refresh should succeed: %v", err)
	}
	if count != 1 {
		t.Fatalf("unexpected active count after refresh: %d", count)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return DecrementNodeNameCountTx(ctx, tx, node)
	}); err != nil {
		t.Fatalf("decrement tx should succeed: %v", err)
	}
	if err := RefreshNodeNameCountCache(ctx, db); err != nil {
		t.Fatalf("refresh count cache should succeed: %v", err)
	}
	count, err = GetNodeNameActiveCount(ctx, db, node.GPUName, node.GPUVram, nodeVersion)
	if err != nil {
		t.Fatalf("get active count should succeed: %v", err)
	}
	if count != 0 {
		t.Fatalf("unexpected active count after decrement: %d", count)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return DecrementNodeNameCountTx(ctx, tx, node)
	}); !errors.Is(err, ErrNodeNameCountEntryMissing) {
		t.Fatalf("expected ErrNodeNameCountEntryMissing, got %v", err)
	}
}
