package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskModelConfigForM20260817 struct {
	ModelName      string `gorm:"size:512;not null;default:''"`
	ModelVariant   string `gorm:"size:191;not null;default:''"`
	RequestedDType string `gorm:"column:requested_dtype;size:64;not null;default:'auto'"`
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null;default:''"`
	QuantizeBits   uint64 `gorm:"not null;default:0"`
}

func (inferenceTaskModelConfigForM20260817) TableName() string { return "inference_tasks" }

type gpuExecutionCalibrationConfigForM20260817 struct {
	TaskType           uint8  `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName            string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram            uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName          string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ModelVariant       string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	ExecutionDType     string `gorm:"column:execution_dtype;size:64;not null;default:'auto';uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	QuantizeBits       uint64 `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
	MinVRAMRequirement uint64 `gorm:"column:min_vram_requirement;not null;default:0"`
	MaxVRAMRequirement uint64 `gorm:"column:max_vram_requirement;not null;default:0"`
}

func (gpuExecutionCalibrationConfigForM20260817) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationLegacyIndexForM20260817 struct {
	GPUName string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_variant,priority:1"`
	GPUVram uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_variant,priority:2"`
}

func (gpuExecutionCalibrationLegacyIndexForM20260817) TableName() string {
	return "gpu_execution_calibrations"
}

var inferenceTaskModelConfigColumnsForM20260817 = []string{
	"ModelName", "ModelVariant", "RequestedDType", "ExecutionDType", "QuantizeBits",
}

var gpuExecutionCalibrationConfigColumnsForM20260817 = []string{
	"TaskType", "ModelName", "ModelVariant", "ExecutionDType", "QuantizeBits",
	"MinVRAMRequirement", "MaxVRAMRequirement",
}

func M20260817(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20260817",
		Migrate: func(tx *gorm.DB) error {
			for _, column := range inferenceTaskModelConfigColumnsForM20260817 {
				if err := tx.Migrator().AddColumn(&inferenceTaskModelConfigForM20260817{}, column); err != nil {
					return err
				}
			}
			for _, column := range gpuExecutionCalibrationConfigColumnsForM20260817 {
				if err := tx.Migrator().AddColumn(&gpuExecutionCalibrationConfigForM20260817{}, column); err != nil {
					return err
				}
			}
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().
				Delete(&gpuExecutionCalibrationConfigForM20260817{}).Error; err != nil {
				return err
			}
			if err := tx.Migrator().DropIndex(&gpuExecutionCalibrationLegacyIndexForM20260817{}, "idx_gpu_execution_calibration_variant"); err != nil {
				return err
			}
			return tx.Migrator().CreateIndex(&gpuExecutionCalibrationConfigForM20260817{}, "idx_gpu_execution_calibration_config")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropIndex(&gpuExecutionCalibrationConfigForM20260817{}, "idx_gpu_execution_calibration_config"); err != nil {
				return err
			}
			for i := len(gpuExecutionCalibrationConfigColumnsForM20260817) - 1; i >= 0; i-- {
				if err := tx.Migrator().DropColumn(&gpuExecutionCalibrationConfigForM20260817{}, gpuExecutionCalibrationConfigColumnsForM20260817[i]); err != nil {
					return err
				}
			}
			if err := tx.Migrator().CreateIndex(&gpuExecutionCalibrationLegacyIndexForM20260817{}, "idx_gpu_execution_calibration_variant"); err != nil {
				return err
			}
			for i := len(inferenceTaskModelConfigColumnsForM20260817) - 1; i >= 0; i-- {
				if err := tx.Migrator().DropColumn(&inferenceTaskModelConfigForM20260817{}, inferenceTaskModelConfigColumnsForM20260817[i]); err != nil {
					return err
				}
			}
			return nil
		},
	}})
}
