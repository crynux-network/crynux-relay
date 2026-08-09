package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskTableForM20260809 struct {
	ID uint `gorm:"primaryKey"`
}

func (inferenceTaskTableForM20260809) TableName() string {
	return "inference_tasks"
}

func TestM20260809AddsWorkloadAndResetsOnlyLLMCalibration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskTableForM20260809{}); err != nil {
		t.Fatalf("create inference tasks: %v", err)
	}
	if err := db.Migrator().CreateTable(&gpuExecutionCalibrationForM20260805{}); err != nil {
		t.Fatalf("create gpu calibrations: %v", err)
	}
	record := gpuExecutionCalibrationForM20260805{
		GPUName: "A100", GPUVram: 40, SecondsPerSDPixelStep: 0.25, SDSuccessSamples: 7,
		LLMConstantSeconds: 12, LLMSecondsPerInputByte: 0.01, LLMSecondsPerOutputToken: 0.2,
		LLMXTX00: 1, LLMXTY0: 2, LLMSuccessSamples: 9,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed calibration: %v", err)
	}

	migration := M20260809(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, column := range inferenceTaskLLMWorkloadColumnsForM20260809 {
		if !db.Migrator().HasColumn(&inferenceTaskLLMWorkloadForM20260809{}, column) {
			t.Fatalf("missing inference task column %s", column)
		}
	}
	var persisted gpuExecutionCalibrationForM20260805
	if err := db.First(&persisted, record.ID).Error; err != nil {
		t.Fatalf("load calibration: %v", err)
	}
	if persisted.SecondsPerSDPixelStep != 0.25 || persisted.SDSuccessSamples != 7 {
		t.Fatalf("SD calibration changed: %+v", persisted)
	}
	if persisted.LLMConstantSeconds != 0 || persisted.LLMSuccessSamples != 0 ||
		persisted.LLMXTX00 != 0 || persisted.LLMXTY0 != 0 {
		t.Fatalf("LLM calibration was not reset: %+v", persisted)
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for _, column := range inferenceTaskLLMWorkloadColumnsForM20260809 {
		if db.Migrator().HasColumn(&inferenceTaskLLMWorkloadForM20260809{}, column) {
			t.Fatalf("inference task column %s remains after rollback", column)
		}
	}
}
