package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeTableForM20260714_1 struct {
	ID      uint   `gorm:"primarykey"`
	GPUName string `gorm:"column:gpu_name;size:191"`
}

func (nodeTableForM20260714_1) TableName() string {
	return "nodes"
}

type nodeNameCountTableForM20260714_1 struct {
	ID          uint   `gorm:"primarykey"`
	GPUName     string `gorm:"column:gpu_name;not null;size:191;uniqueIndex:idx_node_name_count_unique"`
	GPUVram     uint64 `gorm:"column:gpu_vram;not null;uniqueIndex:idx_node_name_count_unique"`
	NodeVersion string `gorm:"column:node_version;not null;size:32;uniqueIndex:idx_node_name_count_unique"`
	ActiveCount uint64 `gorm:"column:active_count;not null"`
}

func (nodeNameCountTableForM20260714_1) TableName() string {
	return "node_name_counts"
}

type nodeNameWhitelistTableForM20260714_1 struct {
	ID          uint   `gorm:"primarykey"`
	GPUName     string `gorm:"column:gpu_name;not null;size:191;uniqueIndex:idx_node_name_whitelist_unique"`
	GPUVram     uint64 `gorm:"column:gpu_vram;not null;uniqueIndex:idx_node_name_whitelist_unique"`
	NodeVersion string `gorm:"column:node_version;not null;size:32;uniqueIndex:idx_node_name_whitelist_unique"`
}

func (nodeNameWhitelistTableForM20260714_1) TableName() string {
	return "node_name_whitelists"
}

type snapshotTableForM20260714_1 struct {
	ID      uint   `gorm:"primarykey"`
	GPUName string `gorm:"column:gpu_name;size:191"`
}

func (snapshotTableForM20260714_1) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func newM20260714_1TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(
		&nodeTableForM20260714_1{},
		&nodeNameCountTableForM20260714_1{},
		&nodeNameWhitelistTableForM20260714_1{},
		&snapshotTableForM20260714_1{},
	); err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}
	return db
}

func TestM20260714_1NormalizesGPUNames(t *testing.T) {
	db := newM20260714_1TestDB(t)

	dirtyDarwinName := " Apple M4\n      Type+Darwin"
	cleanDarwinName := "Apple M4 Type+Darwin"
	cleanName := "NVIDIA GeForce RTX 4090"

	nodes := []nodeTableForM20260714_1{
		{GPUName: dirtyDarwinName},
		{GPUName: cleanName},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatalf("failed to seed nodes: %v", err)
	}

	counts := []nodeNameCountTableForM20260714_1{
		{GPUName: dirtyDarwinName, GPUVram: 16, NodeVersion: "2.6.0", ActiveCount: 3},
		{GPUName: cleanDarwinName, GPUVram: 16, NodeVersion: "2.6.0", ActiveCount: 2},
		{GPUName: cleanName, GPUVram: 24, NodeVersion: "2.6.0", ActiveCount: 5},
	}
	if err := db.Create(&counts).Error; err != nil {
		t.Fatalf("failed to seed node_name_counts: %v", err)
	}

	whitelists := []nodeNameWhitelistTableForM20260714_1{
		{GPUName: dirtyDarwinName, GPUVram: 16, NodeVersion: "2.6.0"},
		{GPUName: cleanDarwinName, GPUVram: 16, NodeVersion: "2.6.0"},
		{GPUName: cleanName, GPUVram: 24, NodeVersion: "2.6.0"},
	}
	if err := db.Create(&whitelists).Error; err != nil {
		t.Fatalf("failed to seed node_name_whitelists: %v", err)
	}

	snapshots := []snapshotTableForM20260714_1{
		{GPUName: dirtyDarwinName},
		{GPUName: cleanName},
	}
	if err := db.Create(&snapshots).Error; err != nil {
		t.Fatalf("failed to seed snapshots: %v", err)
	}

	migration := M20260714_1(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var migratedNodes []nodeTableForM20260714_1
	if err := db.Order("id ASC").Find(&migratedNodes).Error; err != nil {
		t.Fatalf("failed to load nodes: %v", err)
	}
	if migratedNodes[0].GPUName != cleanDarwinName {
		t.Fatalf("unexpected node gpu_name: %q", migratedNodes[0].GPUName)
	}
	if migratedNodes[1].GPUName != cleanName {
		t.Fatalf("clean node gpu_name should be unchanged: %q", migratedNodes[1].GPUName)
	}

	var migratedCounts []nodeNameCountTableForM20260714_1
	if err := db.Order("gpu_name ASC").Find(&migratedCounts).Error; err != nil {
		t.Fatalf("failed to load node_name_counts: %v", err)
	}
	if len(migratedCounts) != 2 {
		t.Fatalf("expected 2 count rows after merge, got %d", len(migratedCounts))
	}
	if migratedCounts[0].GPUName != cleanDarwinName || migratedCounts[0].ActiveCount != 5 {
		t.Fatalf("unexpected merged darwin count row: %+v", migratedCounts[0])
	}
	if migratedCounts[1].GPUName != cleanName || migratedCounts[1].ActiveCount != 5 {
		t.Fatalf("unexpected clean count row: %+v", migratedCounts[1])
	}

	var migratedWhitelists []nodeNameWhitelistTableForM20260714_1
	if err := db.Order("gpu_name ASC").Find(&migratedWhitelists).Error; err != nil {
		t.Fatalf("failed to load node_name_whitelists: %v", err)
	}
	if len(migratedWhitelists) != 2 {
		t.Fatalf("expected 2 whitelist rows after dedupe, got %d", len(migratedWhitelists))
	}
	if migratedWhitelists[0].GPUName != cleanDarwinName {
		t.Fatalf("unexpected darwin whitelist row: %+v", migratedWhitelists[0])
	}
	if migratedWhitelists[1].GPUName != cleanName {
		t.Fatalf("unexpected clean whitelist row: %+v", migratedWhitelists[1])
	}

	var migratedSnapshots []snapshotTableForM20260714_1
	if err := db.Order("id ASC").Find(&migratedSnapshots).Error; err != nil {
		t.Fatalf("failed to load snapshots: %v", err)
	}
	if migratedSnapshots[0].GPUName != cleanDarwinName {
		t.Fatalf("unexpected snapshot gpu_name: %q", migratedSnapshots[0].GPUName)
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
}
