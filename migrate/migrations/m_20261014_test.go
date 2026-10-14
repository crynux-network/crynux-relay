package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskForM20261014Test struct {
	ID           uint   `gorm:"primaryKey"`
	QuantizeBits uint64 `gorm:"not null;default:0"`
}

func (inferenceTaskForM20261014Test) TableName() string { return "inference_tasks" }

type gpuExecutionCalibrationForM20261014Test struct {
	ID                     uint   `gorm:"primaryKey"`
	QuantizationConfigHash []byte `gorm:"column:quantization_config_hash;type:binary(32);not null"`
	QuantizeBits           uint64 `gorm:"not null;default:0"`
}

func (gpuExecutionCalibrationForM20261014Test) TableName() string {
	return "gpu_execution_calibrations"
}

func TestM20261014DropsQuantizeBitsWithRollback(t *testing.T) {
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
	if err := db.Migrator().CreateTable(&inferenceTaskForM20261014Test{}, &gpuExecutionCalibrationForM20261014Test{}); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	migration := M20261014(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	migrator := db.Migrator()
	if migrator.HasColumn(&inferenceTaskQuantizeBitsForM20261014{}, "QuantizeBits") {
		t.Fatal("inference_tasks.quantize_bits remains after migrate")
	}
	if migrator.HasColumn(&gpuExecutionCalibrationQuantizeBitsForM20261014{}, "QuantizeBits") {
		t.Fatal("gpu_execution_calibrations.quantize_bits remains after migrate")
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !migrator.HasColumn(&inferenceTaskQuantizeBitsForM20261014{}, "QuantizeBits") {
		t.Fatal("inference_tasks.quantize_bits is missing after rollback")
	}
	if !migrator.HasColumn(&gpuExecutionCalibrationQuantizeBitsForM20261014{}, "QuantizeBits") {
		t.Fatal("gpu_execution_calibrations.quantize_bits is missing after rollback")
	}
}
