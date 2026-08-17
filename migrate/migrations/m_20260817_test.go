package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260817RekeysAndClearsCalibrationsWithRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskTableForM20260809{}); err != nil {
		t.Fatalf("create inference tasks: %v", err)
	}
	if err := db.Migrator().CreateTable(&gpuExecutionCalibrationForM20260805{}); err != nil {
		t.Fatalf("create calibrations: %v", err)
	}
	if err := db.Create(&gpuExecutionCalibrationForM20260805{GPUName: "A100", GPUVram: 40}).Error; err != nil {
		t.Fatalf("seed calibration: %v", err)
	}

	migration := M20260817(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var count int64
	if err := db.Table("gpu_execution_calibrations").Count(&count).Error; err != nil {
		t.Fatalf("count calibrations: %v", err)
	}
	if count != 0 {
		t.Fatalf("calibrations were not cleared: %d", count)
	}
	if !db.Migrator().HasIndex(&gpuExecutionCalibrationConfigForM20260817{}, "idx_gpu_execution_calibration_config") {
		t.Fatal("new unique index is missing")
	}
	for _, column := range inferenceTaskModelConfigColumnsForM20260817 {
		if !db.Migrator().HasColumn(&inferenceTaskModelConfigForM20260817{}, column) {
			t.Fatalf("missing task column %s", column)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !db.Migrator().HasIndex(&gpuExecutionCalibrationLegacyIndexForM20260817{}, "idx_gpu_execution_calibration_variant") {
		t.Fatal("legacy unique index was not restored")
	}
	for _, column := range inferenceTaskModelConfigColumnsForM20260817 {
		if db.Migrator().HasColumn(&inferenceTaskModelConfigForM20260817{}, column) {
			t.Fatalf("task column %s remains after rollback", column)
		}
	}
}
