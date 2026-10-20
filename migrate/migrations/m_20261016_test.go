package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskForM20261016Test struct {
	ID uint `gorm:"primaryKey"`
}

func (inferenceTaskForM20261016Test) TableName() string { return "inference_tasks" }

func TestM20261016AddsRequireLinuxCudaAndConstraintTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Migrator().CreateTable(&inferenceTaskForM20261016Test{}); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	migration := M20261016(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	migrator := db.Migrator()
	if !migrator.HasColumn(&inferenceTaskRequireLinuxCudaForM20261016{}, "RequireLinuxCuda") {
		t.Fatal("expected require_linux_cuda column")
	}
	if !migrator.HasTable(&modelRuntimeConstraintForM20261016{}) {
		t.Fatal("expected model_runtime_constraints table")
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if migrator.HasColumn(&inferenceTaskRequireLinuxCudaForM20261016{}, "RequireLinuxCuda") {
		t.Fatal("expected require_linux_cuda column to be dropped")
	}
	if migrator.HasTable(&modelRuntimeConstraintForM20261016{}) {
		t.Fatal("expected model_runtime_constraints table to be dropped")
	}
}
