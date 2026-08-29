package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260828AddsTaskBatchFieldsAndIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskBatchFieldsForM20260828{}); err != nil {
		t.Fatalf("create inference task table: %v", err)
	}
	for _, column := range []string{"execution_gpu", "execution_gpu_vram", "estimated_completion_time", "deadline_at"} {
		if err := db.Migrator().DropColumn(&inferenceTaskBatchFieldsForM20260828{}, column); err != nil {
			t.Fatalf("drop preexisting column %s: %v", column, err)
		}
	}

	migration := M20260828(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasIndex(&inferenceTaskBatchFieldsForM20260828{}, "idx_inference_tasks_status_deadline_id") {
		t.Fatal("missing deadline index")
	}
}
