package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260709_1CreatesLoadedModelsTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	migration := M20260709_1(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasTable(&loadedModelMigration{}) {
		t.Fatalf("expected loaded_models table to exist")
	}

	loadedModel := loadedModelMigration{ModelID: "qwen/qwen3.6-7b", MinVRAM: 16}
	if err := db.Create(&loadedModel).Error; err != nil {
		t.Fatalf("failed to create loaded model: %v", err)
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasTable(&loadedModelMigration{}) {
		t.Fatalf("expected loaded_models table to be dropped")
	}
}

func TestM20260709_1SkipsExistingLoadedModelsTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&loadedModelMigration{}); err != nil {
		t.Fatalf("failed to pre-create loaded_models table: %v", err)
	}

	migration := M20260709_1(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed on existing table: %v", err)
	}
	if !db.Migrator().HasTable(&loadedModelMigration{}) {
		t.Fatalf("expected loaded_models table to exist")
	}
}
