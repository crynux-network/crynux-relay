package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeModelForHFModelIDMigrationTest struct {
	ID          uint `gorm:"primaryKey"`
	NodeAddress string
	ModelID     string
	InUse       bool
}

func (nodeModelForHFModelIDMigrationTest) TableName() string {
	return "node_models"
}

func TestM20260713BackfillsHFModelID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&nodeModelForHFModelIDMigrationTest{}); err != nil {
		t.Fatalf("failed to create node_models table: %v", err)
	}
	rows := []nodeModelForHFModelIDMigrationTest{
		{NodeAddress: "0x1", ModelID: "base:qwen/qwen3-8b", InUse: true},
		{NodeAddress: "0x1", ModelID: "base:meta/llama+fp16", InUse: false},
		{NodeAddress: "0x2", ModelID: "lora:crynux-network/mylora", InUse: false},
		{NodeAddress: "0x2", ModelID: "base:https://example.com/model.safetensors", InUse: false},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to create node models: %v", err)
	}

	migration := M20260713(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&nodeModelHFModelIDMigration{}, "HFModelID") {
		t.Fatalf("expected hf_model_id column to exist")
	}
	if !db.Migrator().HasIndex(&nodeModelHFModelIDMigration{}, "idx_node_models_hf_model_id_node_address") {
		t.Fatalf("expected composite index to exist")
	}

	expected := map[string]string{
		"base:qwen/qwen3-8b":                         "qwen/qwen3-8b",
		"base:meta/llama+fp16":                       "meta/llama",
		"lora:crynux-network/mylora":                 "",
		"base:https://example.com/model.safetensors": "",
	}
	for modelID, expectedHFModelID := range expected {
		var hfModelID string
		if err := db.Table("node_models").
			Where("model_id = ?", modelID).
			Pluck("hf_model_id", &hfModelID).Error; err != nil {
			t.Fatalf("failed to query hf_model_id for %q: %v", modelID, err)
		}
		if hfModelID != expectedHFModelID {
			t.Fatalf("unexpected hf_model_id for %q: got %q, want %q", modelID, hfModelID, expectedHFModelID)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&nodeModelHFModelIDMigration{}, "HFModelID") {
		t.Fatalf("expected hf_model_id column to be dropped")
	}
}
