package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeModelDownloadSelectionTableForM20260717 struct {
	ID uint `gorm:"primarykey"`
}

func (nodeModelDownloadSelectionTableForM20260717) TableName() string {
	return "node_model_download_selections"
}

func TestM20260717AddsMinVRAM(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&nodeModelDownloadSelectionTableForM20260717{}); err != nil {
		t.Fatalf("failed to create node_model_download_selections table: %v", err)
	}

	migration := M20260717(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&nodeModelDownloadSelectionMinVRAMMigration{}, "MinVRAM") {
		t.Fatalf("expected node_model_download_selections.min_vram column to exist")
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&nodeModelDownloadSelectionMinVRAMMigration{}, "MinVRAM") {
		t.Fatalf("expected node_model_download_selections.min_vram column to be dropped")
	}
}
