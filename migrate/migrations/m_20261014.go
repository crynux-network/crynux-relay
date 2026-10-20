package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskQuantizeBitsForM20261014 struct {
	QuantizeBits uint64 `gorm:"not null;default:0"`
}

func (inferenceTaskQuantizeBitsForM20261014) TableName() string { return "inference_tasks" }

type gpuExecutionCalibrationQuantizeBitsForM20261014 struct {
	QuantizeBits uint64 `gorm:"not null;default:0"`
}

func (gpuExecutionCalibrationQuantizeBitsForM20261014) TableName() string {
	return "gpu_execution_calibrations"
}

// M20261014 drops the quantize_bits columns. Quantization identity is carried
// by quantization_config_hash.
func M20261014(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261014",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropColumn(&gpuExecutionCalibrationQuantizeBitsForM20261014{}, "QuantizeBits"); err != nil {
				return err
			}
			return tx.Migrator().DropColumn(&inferenceTaskQuantizeBitsForM20261014{}, "QuantizeBits")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().AddColumn(&inferenceTaskQuantizeBitsForM20261014{}, "QuantizeBits"); err != nil {
				return err
			}
			return tx.Migrator().AddColumn(&gpuExecutionCalibrationQuantizeBitsForM20261014{}, "QuantizeBits")
		},
	}})
}
