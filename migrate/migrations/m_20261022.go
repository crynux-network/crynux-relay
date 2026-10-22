package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type loadedModelWithVariantForM20261022 struct {
	ID                      uint `gorm:"primaryKey"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	RequiredModelFilesHash  []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:1"`
	ModelCacheHash          []byte `gorm:"column:model_cache_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:2"`
	ExecutionDType          string `gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_loaded_models_identity,priority:3"`
	RequiredModelFileHashes []byte `gorm:"type:longblob;not null"`
	QuantizationConfig      string `gorm:"type:json;not null"`
	ModelID                 string `gorm:"not null;size:191"`
	Variant                 string `gorm:"not null;size:64;default:''"`
	ModelType               string `gorm:"not null;size:16"`
	MinVRAM                 uint64 `gorm:"column:min_vram;not null;index"`
}

func (loadedModelWithVariantForM20261022) TableName() string { return "loaded_models" }

type loadedModelForM20261022 struct {
	ID                      uint `gorm:"primaryKey"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	RequiredModelFilesHash  []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:1"`
	ModelCacheHash          []byte `gorm:"column:model_cache_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:2"`
	ExecutionDType          string `gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_loaded_models_identity,priority:3"`
	RequiredModelFileHashes []byte `gorm:"type:longblob;not null"`
	QuantizationConfig      string `gorm:"type:json;not null"`
	ModelID                 string `gorm:"not null;size:191"`
	ModelCacheConfig        string `gorm:"column:model_cache_config;type:json;not null"`
	ModelType               string `gorm:"not null;size:16"`
	MinVRAM                 uint64 `gorm:"column:min_vram;not null;index"`
}

func (loadedModelForM20261022) TableName() string { return "loaded_models" }

type inferenceTaskModelVariantForM20261022 struct {
	ModelVariant string `gorm:"size:191;not null;default:''"`
}

func (inferenceTaskModelVariantForM20261022) TableName() string { return "inference_tasks" }

type gpuExecutionCalibrationRowsForM20261022 struct {
	ID uint `gorm:"primaryKey"`
}

func (gpuExecutionCalibrationRowsForM20261022) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationModelVariantForM20261022 struct {
	ModelVariant string `gorm:"size:191;not null;default:''"`
}

func (gpuExecutionCalibrationModelVariantForM20261022) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationKeyWithVariantForM20261022 struct {
	TaskType               uint8  `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName                string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram                uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName              string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ModelVariant           string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	ExecutionDType         string `gorm:"column:execution_dtype;size:64;not null;default:'auto';uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
	QuantizationConfigHash []byte `gorm:"column:quantization_config_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:8"`
}

func (gpuExecutionCalibrationKeyWithVariantForM20261022) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationKeyForM20261022 struct {
	TaskType               uint8  `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName                string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram                uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName              string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ExecutionDType         string `gorm:"column:execution_dtype;size:64;not null;default:'auto';uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	QuantizationConfigHash []byte `gorm:"column:quantization_config_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
}

func (gpuExecutionCalibrationKeyForM20261022) TableName() string {
	return "gpu_execution_calibrations"
}

// M20261022 moves loaded model identity to the model cache config and removes
// the generic model variant. Loaded models and calibrations are rebuilt empty
// because their previous keys cannot be mapped to the new identity.
func M20261022(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261022",
		Migrate: func(tx *gorm.DB) error {
			if err := replaceTableForM20261013(tx, &loadedModelWithVariantForM20261022{}, &loadedModelForM20261022{}); err != nil {
				return err
			}
			if err := tx.Migrator().DropColumn(&inferenceTaskModelVariantForM20261022{}, "ModelVariant"); err != nil {
				return err
			}
			if err := clearCalibrationsForM20261022(tx); err != nil {
				return err
			}
			if err := tx.Migrator().DropIndex(&gpuExecutionCalibrationKeyWithVariantForM20261022{}, "idx_gpu_execution_calibration_config"); err != nil {
				return err
			}
			if err := tx.Migrator().DropColumn(&gpuExecutionCalibrationModelVariantForM20261022{}, "ModelVariant"); err != nil {
				return err
			}
			return tx.Migrator().CreateIndex(&gpuExecutionCalibrationKeyForM20261022{}, "idx_gpu_execution_calibration_config")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := clearCalibrationsForM20261022(tx); err != nil {
				return err
			}
			if err := tx.Migrator().DropIndex(&gpuExecutionCalibrationKeyForM20261022{}, "idx_gpu_execution_calibration_config"); err != nil {
				return err
			}
			if err := tx.Migrator().AddColumn(&gpuExecutionCalibrationModelVariantForM20261022{}, "ModelVariant"); err != nil {
				return err
			}
			if err := tx.Migrator().CreateIndex(&gpuExecutionCalibrationKeyWithVariantForM20261022{}, "idx_gpu_execution_calibration_config"); err != nil {
				return err
			}
			if err := tx.Migrator().AddColumn(&inferenceTaskModelVariantForM20261022{}, "ModelVariant"); err != nil {
				return err
			}
			return replaceTableForM20261013(tx, &loadedModelForM20261022{}, &loadedModelWithVariantForM20261022{})
		},
	}})
}

func clearCalibrationsForM20261022(tx *gorm.DB) error {
	return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Delete(&gpuExecutionCalibrationRowsForM20261022{}).Error
}
