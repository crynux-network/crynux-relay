package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260819AddsSDFitColumnsWithRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&gpuExecutionCalibrationForM20260805{}); err != nil {
		t.Fatalf("create calibrations: %v", err)
	}

	migration := M20260819(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, column := range gpuExecutionCalibrationSDFitColumnsForM20260819 {
		if !db.Migrator().HasColumn(&gpuExecutionCalibrationSDFitForM20260819{}, column) {
			t.Fatalf("missing column %s", column)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for _, column := range gpuExecutionCalibrationSDFitColumnsForM20260819 {
		if db.Migrator().HasColumn(&gpuExecutionCalibrationSDFitForM20260819{}, column) {
			t.Fatalf("column %s remains after rollback", column)
		}
	}
}
