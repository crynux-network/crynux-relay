package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeModelForSoftDeleteCleanupTest struct {
	ID          uint `gorm:"primaryKey"`
	NodeAddress string
	ModelID     string
	HFModelID   string `gorm:"column:hf_model_id;not null;default:''"`
	InUse       bool
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (nodeModelForSoftDeleteCleanupTest) TableName() string {
	return "node_models"
}

func TestM20260713_1PurgesSoftDeletedRowsAndLowercasesHFModelID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&nodeModelForSoftDeleteCleanupTest{}); err != nil {
		t.Fatalf("failed to create node_models table: %v", err)
	}
	rows := []nodeModelForSoftDeleteCleanupTest{
		{NodeAddress: "0x1", ModelID: "base:qwen/qwen3-8b", HFModelID: "Qwen/Qwen3-8B"},
		{NodeAddress: "0x2", ModelID: "base:meta/llama", HFModelID: "meta/llama"},
		{NodeAddress: "0x3", ModelID: "lora:crynux-network/mylora", HFModelID: ""},
		{NodeAddress: "0x4", ModelID: "base:Qwen/Qwen3-8B", HFModelID: "Qwen/Qwen3-8B", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to create node models: %v", err)
	}

	migration := M20260713_1(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	if db.Migrator().HasColumn(&nodeModelSoftDeleteCleanupMigration{}, "DeletedAt") {
		t.Fatalf("expected deleted_at column to be dropped")
	}
	var count int64
	if err := db.Table("node_models").Count(&count).Error; err != nil {
		t.Fatalf("failed to count node models: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 rows after purging soft-deleted rows, got %d", count)
	}
	expected := map[uint]string{
		rows[0].ID: "qwen/qwen3-8b",
		rows[1].ID: "meta/llama",
		rows[2].ID: "",
	}
	for id, expectedHFModelID := range expected {
		var hfModelID string
		if err := db.Table("node_models").
			Where("id = ?", id).
			Pluck("hf_model_id", &hfModelID).Error; err != nil {
			t.Fatalf("failed to query hf_model_id for row %d: %v", id, err)
		}
		if hfModelID != expectedHFModelID {
			t.Fatalf("unexpected hf_model_id for row %d: got %q, want %q", id, hfModelID, expectedHFModelID)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !db.Migrator().HasColumn(&nodeModelSoftDeleteCleanupMigration{}, "DeletedAt") {
		t.Fatalf("expected deleted_at column to be restored")
	}
}
