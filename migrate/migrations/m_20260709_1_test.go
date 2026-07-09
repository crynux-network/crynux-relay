package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type loadedModelBackfillTaskTest struct {
	ID           uint `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ModelIDs     string `gorm:"type:text"`
	Status       int
	SelectedNode string
}

func (loadedModelBackfillTaskTest) TableName() string {
	return "inference_tasks"
}

type loadedModelBackfillNodeTest struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Address   string `gorm:"index"`
	GPUVram   uint64
}

func (loadedModelBackfillNodeTest) TableName() string {
	return "nodes"
}

func TestM20260709_1BackfillsLoadedModels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&loadedModelBackfillTaskTest{}, &loadedModelBackfillNodeTest{}); err != nil {
		t.Fatalf("failed to migrate source tables: %v", err)
	}

	nodes := []loadedModelBackfillNodeTest{
		{Address: "node-a", GPUVram: 24},
		{Address: "node-b", GPUVram: 16},
		{Address: "node-c", GPUVram: 32},
		{Address: "node-d", GPUVram: 8},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatalf("failed to create nodes: %v", err)
	}
	tasks := []loadedModelBackfillTaskTest{
		{ModelIDs: "qwen/qwen3.6-7b;meta/llama", Status: loadedModelTaskEndSuccess, SelectedNode: "node-a"},
		{ModelIDs: "qwen/qwen3.6-7b", Status: loadedModelTaskEndGroupRefund, SelectedNode: "node-b"},
		{ModelIDs: "meta/llama", Status: loadedModelTaskEndGroupSuccess, SelectedNode: "node-c"},
		{ModelIDs: "ignored/model", Status: 7, SelectedNode: "node-d"},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatalf("failed to create tasks: %v", err)
	}

	migration := M20260709_1(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var loadedModels []loadedModelMigration
	if err := db.Order("model_id ASC").Find(&loadedModels).Error; err != nil {
		t.Fatalf("failed to query loaded models: %v", err)
	}
	if len(loadedModels) != 2 {
		t.Fatalf("expected 2 loaded models, got %d", len(loadedModels))
	}
	if loadedModels[0].ModelID != "meta/llama" || loadedModels[0].MinVRAM != 24 {
		t.Fatalf("unexpected first loaded model: %+v", loadedModels[0])
	}
	if loadedModels[1].ModelID != "qwen/qwen3.6-7b" || loadedModels[1].MinVRAM != 16 {
		t.Fatalf("unexpected second loaded model: %+v", loadedModels[1])
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasTable(&loadedModelMigration{}) {
		t.Fatalf("expected loaded_models table to be dropped")
	}
}
