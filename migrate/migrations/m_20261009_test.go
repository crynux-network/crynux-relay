package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20261009AddsVariantBackfillsSDAndCompositeUnique(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:m20261009?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := M20260709_1(db).Migrate(); err != nil {
		t.Fatalf("prerequisite migration M20260709_1 failed: %v", err)
	}
	if err := M20260709_2(db).Migrate(); err != nil {
		t.Fatalf("prerequisite migration M20260709_2 failed: %v", err)
	}

	now := time.Now()
	rows := []loadedModelVariantForM20261009{
		{CreatedAt: now, UpdatedAt: now, ModelID: "crynux-network/sdxl-turbo", ModelType: "sd", MinVRAM: 14},
		{CreatedAt: now, UpdatedAt: now, ModelID: "qwen/qwen3.6-7b", ModelType: "llm", MinVRAM: 24},
	}
	if err := db.Omit("Variant").Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed loaded models: %v", err)
	}

	migration := M20261009(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&loadedModelVariantForM20261009{}, "Variant") {
		t.Fatal("expected variant column")
	}
	if !db.Migrator().HasIndex(&loadedModelCompositeUniqueForM20261009{}, "idx_loaded_models_model_id_variant") {
		t.Fatal("expected composite unique index")
	}
	if db.Migrator().HasIndex(&loadedModelOldUniqueForM20261009{}, "idx_loaded_models_model_id") {
		t.Fatal("expected old unique index to be dropped")
	}

	var sd loadedModelVariantForM20261009
	if err := db.Where("model_id = ?", "crynux-network/sdxl-turbo").First(&sd).Error; err != nil {
		t.Fatalf("failed to load sd row: %v", err)
	}
	if sd.Variant != "fp16" {
		t.Fatalf("expected sd variant fp16, got %q", sd.Variant)
	}
	var llm loadedModelVariantForM20261009
	if err := db.Where("model_id = ?", "qwen/qwen3.6-7b").First(&llm).Error; err != nil {
		t.Fatalf("failed to load llm row: %v", err)
	}
	if llm.Variant != "" {
		t.Fatalf("expected llm variant empty, got %q", llm.Variant)
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&loadedModelVariantForM20261009{}, "Variant") {
		t.Fatal("expected variant column dropped on rollback")
	}
}
