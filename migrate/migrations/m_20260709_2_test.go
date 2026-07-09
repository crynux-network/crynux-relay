package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260709_2AddsModelTypeAndClearsRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := M20260709_1(db).Migrate(); err != nil {
		t.Fatalf("prerequisite migration failed: %v", err)
	}
	if err := db.Create(&loadedModelMigration{ModelID: "base:qwen/qwen3-8b", MinVRAM: 24}).Error; err != nil {
		t.Fatalf("failed to create loaded model: %v", err)
	}

	migration := M20260709_2(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&loadedModelModelTypeMigration{}, "ModelType") {
		t.Fatalf("expected model_type column to exist")
	}
	var count int64
	if err := db.Table("loaded_models").Count(&count).Error; err != nil {
		t.Fatalf("failed to count loaded models: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected loaded_models to be empty after migration, got %d rows", count)
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&loadedModelModelTypeMigration{}, "ModelType") {
		t.Fatalf("expected model_type column to be dropped")
	}
}
