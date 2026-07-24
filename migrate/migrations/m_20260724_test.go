package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260724CreatesAndDropsNodeTaskErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	migration := M20260724(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasTable(&nodeTaskErrorForM20260724{}) {
		t.Fatal("expected node_task_errors table to exist")
	}
	for _, index := range []string{
		"idx_node_task_errors_node_address",
		"idx_node_task_errors_task_id_commitment",
		"idx_node_task_errors_node_task",
		"idx_node_task_errors_created_id",
	} {
		if !db.Migrator().HasIndex(&nodeTaskErrorForM20260724{}, index) {
			t.Fatalf("expected index %s to exist", index)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasTable(&nodeTaskErrorForM20260724{}) {
		t.Fatal("expected node_task_errors table to be dropped")
	}
}
