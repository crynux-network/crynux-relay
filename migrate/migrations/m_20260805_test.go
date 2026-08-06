package migrations

import (
	"crynux_relay/models"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskTableForM20260805 struct {
	ID uint `gorm:"primaryKey"`
}

func (inferenceTaskTableForM20260805) TableName() string {
	return "inference_tasks"
}

func TestM20260805CalibrationTableAndTaskWorkloads(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskTableForM20260805{}); err != nil {
		t.Fatalf("create inference_tasks: %v", err)
	}
	migration := M20260805(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasTable(&models.GPUExecutionCalibration{}) {
		t.Fatal("expected gpu_execution_calibrations table")
	}
	for _, column := range []string{"SDUnits", "LLMInputBytes"} {
		if !db.Migrator().HasColumn(&inferenceTaskWorkloadForM20260805{}, column) {
			t.Fatalf("expected inference_tasks.%s", column)
		}
	}
	if !db.Migrator().HasIndex(&models.GPUExecutionCalibration{}, "idx_gpu_execution_calibration_variant") {
		t.Fatal("expected exact GPU unique index")
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if db.Migrator().HasTable(&models.GPUExecutionCalibration{}) {
		t.Fatal("expected calibration table removed")
	}
}
