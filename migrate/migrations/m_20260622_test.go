package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeModelForLowercaseMigrationTest struct {
	gorm.Model
	NodeAddress string `gorm:"index"`
	ModelID     string `gorm:"index"`
	InUse       bool
}

func (nodeModelForLowercaseMigrationTest) TableName() string {
	return "node_models"
}

func TestM20260622LowercasesNonDeletedNodeModelsOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&nodeModelForLowercaseMigrationTest{}); err != nil {
		t.Fatalf("failed to migrate test table: %v", err)
	}

	active := nodeModelForLowercaseMigrationTest{NodeAddress: "0x1", ModelID: "BaSe:Qwen/Qwen3.5-9B+FP16", InUse: false}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("failed to seed active node model: %v", err)
	}
	deleted := nodeModelForLowercaseMigrationTest{NodeAddress: "0x2", ModelID: "LoRa:Crynux-Network/MyLora+V1", InUse: false}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatalf("failed to seed deleted node model: %v", err)
	}
	if err := db.Unscoped().Model(&nodeModelForLowercaseMigrationTest{}).Where("id = ?", deleted.ID).Update("deleted_at", time.Now()).Error; err != nil {
		t.Fatalf("failed to soft-delete node model row: %v", err)
	}

	if err := M20260622(db).Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var rows []nodeModelForLowercaseMigrationTest
	if err := db.Unscoped().Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("failed to load node model rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("unexpected row count: got %d, want %d", len(rows), 2)
	}

	if rows[0].ModelID != "base:qwen/qwen3.5-9b+fp16" {
		t.Fatalf("expected active row model_id to be lowercased, got %q", rows[0].ModelID)
	}
	if rows[1].ModelID != "LoRa:Crynux-Network/MyLora+V1" {
		t.Fatalf("expected soft-deleted row model_id to remain unchanged, got %q", rows[1].ModelID)
	}
}
