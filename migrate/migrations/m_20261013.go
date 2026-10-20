package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const (
	nodeStatusAvailableForM20261013 = 1
	nodeStatusBusyForM20261013      = 2
)

type modelFileForM20261013 struct {
	ModelFileHash       []byte `gorm:"type:binary(32);primaryKey"`
	ModelFileSourceHash []byte `gorm:"type:binary(32);not null;index"`
	CanonicalJSON       string `gorm:"type:json;not null"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (modelFileForM20261013) TableName() string { return "model_files" }

type nodeInMemoryModelForM20261013 struct {
	NodeAddress             string `gorm:"size:191;primaryKey"`
	CurrentModelCacheHash   []byte `gorm:"type:binary(32);not null"`
	CurrentModelFileHashes  []byte `gorm:"type:longblob;not null"`
	CurrentModelCacheConfig string `gorm:"type:json;not null"`
	ExecutionDType          string `gorm:"column:execution_dtype;size:64;not null"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (nodeInMemoryModelForM20261013) TableName() string { return "node_in_memory_models" }

type nodeModelForM20261013 struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	NodeAddress   string `gorm:"size:191;not null;uniqueIndex:idx_node_models_node_address_model_file_hash,priority:1"`
	ModelFileHash []byte `gorm:"type:binary(32);not null;index;uniqueIndex:idx_node_models_node_address_model_file_hash,priority:2"`
}

func (nodeModelForM20261013) TableName() string { return "node_models" }

type nodeModelLegacyForM20261013 struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	NodeAddress string `gorm:"index;index:idx_node_models_hf_model_id_node_address,priority:2"`
	ModelID     string `gorm:"index"`
	HFModelID   string `gorm:"column:hf_model_id;not null;default:'';size:191;index:idx_node_models_hf_model_id_node_address,priority:1"`
	InUse       bool
}

func (nodeModelLegacyForM20261013) TableName() string { return "node_models" }

type loadedModelForM20261013 struct {
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

func (loadedModelForM20261013) TableName() string { return "loaded_models" }

type loadedModelLegacyForM20261013 struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ModelID   string `gorm:"not null;size:191;uniqueIndex:idx_loaded_models_model_id_variant,priority:1"`
	Variant   string `gorm:"not null;size:64;default:'';uniqueIndex:idx_loaded_models_model_id_variant,priority:2"`
	ModelType string `gorm:"not null;size:16"`
	MinVRAM   uint64 `gorm:"column:min_vram;not null;index"`
}

func (loadedModelLegacyForM20261013) TableName() string { return "loaded_models" }

type nodeModelDownloadSelectionForM20261013 struct {
	ID                            uint `gorm:"primaryKey"`
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	ModelFileHash                 []byte `gorm:"type:binary(32);not null;uniqueIndex:idx_node_model_download_selection_active,priority:1"`
	RequiredModelFileHashes       []byte `gorm:"type:longblob;not null"`
	RequiredModelFileSourceHashes []byte `gorm:"type:longblob;not null"`
	NodeAddress                   string `gorm:"size:42;index;uniqueIndex:idx_node_model_download_selection_active,priority:2"`
	MinVRAM                       uint64 `gorm:"column:min_vram;not null;default:0"`
	SentAt                        time.Time
	Deadline                      time.Time
	Status                        string `gorm:"size:16;index"`
	Active                        *bool  `gorm:"uniqueIndex:idx_node_model_download_selection_active,priority:3"`
}

func (nodeModelDownloadSelectionForM20261013) TableName() string {
	return "node_model_download_selections"
}

type nodeModelDownloadSelectionLegacyForM20261013 struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ModelID     string `gorm:"size:255;uniqueIndex:idx_node_model_download_selection_active,priority:1"`
	NodeAddress string `gorm:"size:42;index;uniqueIndex:idx_node_model_download_selection_active,priority:2"`
	MinVRAM     uint64 `gorm:"column:min_vram;not null;default:0"`
	SentAt      time.Time
	Deadline    time.Time
	Status      string `gorm:"size:16;index"`
	Active      *bool  `gorm:"uniqueIndex:idx_node_model_download_selection_active,priority:3"`
}

func (nodeModelDownloadSelectionLegacyForM20261013) TableName() string {
	return "node_model_download_selections"
}

type inferenceTaskRowsForM20261013 struct {
	ID uint `gorm:"primaryKey"`
}

func (inferenceTaskRowsForM20261013) TableName() string { return "inference_tasks" }

type nodeTaskRefForM20261013 struct {
	ID                      uint `gorm:"primaryKey"`
	Status                  uint8
	CurrentTaskIDCommitment *string
}

func (nodeTaskRefForM20261013) TableName() string { return "nodes" }

// inferenceTaskModelFilesForM20261013 adds the columns as nullable; the
// required ones are tightened afterwards because a NOT NULL column without a
// default cannot be added in one step on every supported database.
type inferenceTaskModelFilesForM20261013 struct {
	RequiredModelFileHashes       []byte `gorm:"type:longblob"`
	RequiredModelFileSourceHashes []byte `gorm:"type:longblob"`
	ExecutionModelFileHashes      []byte `gorm:"type:longblob;null"`
	RequestedModelCacheHash       []byte `gorm:"type:binary(32)"`
	QuantizationConfigHash        []byte `gorm:"type:binary(32)"`
}

func (inferenceTaskModelFilesForM20261013) TableName() string { return "inference_tasks" }

type inferenceTaskRequiredModelFilesForM20261013 struct {
	RequiredModelFileHashes       []byte `gorm:"type:longblob;not null"`
	RequiredModelFileSourceHashes []byte `gorm:"type:longblob;not null"`
	RequestedModelCacheHash       []byte `gorm:"type:binary(32);not null"`
	QuantizationConfigHash        []byte `gorm:"type:binary(32);not null"`
}

func (inferenceTaskRequiredModelFilesForM20261013) TableName() string { return "inference_tasks" }

type gpuExecutionCalibrationHashesForM20261013 struct {
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32)"`
	QuantizationConfigHash []byte `gorm:"column:quantization_config_hash;type:binary(32)"`
}

func (gpuExecutionCalibrationHashesForM20261013) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationRequiredHashesForM20261013 struct {
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32);not null"`
	QuantizationConfigHash []byte `gorm:"column:quantization_config_hash;type:binary(32);not null"`
}

func (gpuExecutionCalibrationRequiredHashesForM20261013) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationKeyForM20261013 struct {
	TaskType               uint8  `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName                string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram                uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName              string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ModelVariant           string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	ExecutionDType         string `gorm:"column:execution_dtype;size:64;not null;default:'auto';uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
	QuantizationConfigHash []byte `gorm:"column:quantization_config_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:8"`
}

func (gpuExecutionCalibrationKeyForM20261013) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationLegacyKeyForM20261013 struct {
	TaskType       uint8  `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName        string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram        uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName      string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ModelVariant   string `gorm:"size:191;not null;default:'';uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null;default:'auto';uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	QuantizeBits   uint64 `gorm:"not null;default:0;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
}

func (gpuExecutionCalibrationLegacyKeyForM20261013) TableName() string {
	return "gpu_execution_calibrations"
}

var inferenceTaskModelFileColumnsForM20261013 = []string{
	"RequiredModelFileHashes", "RequiredModelFileSourceHashes", "ExecutionModelFileHashes",
	"RequestedModelCacheHash", "QuantizationConfigHash",
}

var inferenceTaskRequiredModelFileColumnsForM20261013 = []string{
	"RequiredModelFileHashes", "RequiredModelFileSourceHashes",
	"RequestedModelCacheHash", "QuantizationConfigHash",
}

var calibrationHashColumnsForM20261013 = []string{"RequiredModelFilesHash", "QuantizationConfigHash"}

// M20261013 cuts over model identity to model-file hashes. Tasks and every
// table keyed by the previous model identity are cleared and rebuilt empty.
func M20261013(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261013",
		Migrate: func(tx *gorm.DB) error {
			if err := clearTasksForM20261013(tx); err != nil {
				return err
			}
			if err := tx.Migrator().CreateTable(&modelFileForM20261013{}, &nodeInMemoryModelForM20261013{}); err != nil {
				return err
			}
			if err := replaceTableForM20261013(tx, &nodeModelLegacyForM20261013{}, &nodeModelForM20261013{}); err != nil {
				return err
			}
			if err := replaceTableForM20261013(tx, &nodeModelDownloadSelectionLegacyForM20261013{}, &nodeModelDownloadSelectionForM20261013{}); err != nil {
				return err
			}
			if err := replaceTableForM20261013(tx, &loadedModelLegacyForM20261013{}, &loadedModelForM20261013{}); err != nil {
				return err
			}
			for _, column := range inferenceTaskModelFileColumnsForM20261013 {
				if err := tx.Migrator().AddColumn(&inferenceTaskModelFilesForM20261013{}, column); err != nil {
					return err
				}
			}
			for _, column := range inferenceTaskRequiredModelFileColumnsForM20261013 {
				if err := tx.Migrator().AlterColumn(&inferenceTaskRequiredModelFilesForM20261013{}, column); err != nil {
					return err
				}
			}
			return migrateCalibrationKeyForM20261013(tx)
		},
		Rollback: func(tx *gorm.DB) error {
			if err := rollbackCalibrationKeyForM20261013(tx); err != nil {
				return err
			}
			for i := len(inferenceTaskModelFileColumnsForM20261013) - 1; i >= 0; i-- {
				if err := tx.Migrator().DropColumn(&inferenceTaskModelFilesForM20261013{}, inferenceTaskModelFileColumnsForM20261013[i]); err != nil {
					return err
				}
			}
			if err := replaceTableForM20261013(tx, &loadedModelForM20261013{}, &loadedModelLegacyForM20261013{}); err != nil {
				return err
			}
			if err := replaceTableForM20261013(tx, &nodeModelDownloadSelectionForM20261013{}, &nodeModelDownloadSelectionLegacyForM20261013{}); err != nil {
				return err
			}
			if err := replaceTableForM20261013(tx, &nodeModelForM20261013{}, &nodeModelLegacyForM20261013{}); err != nil {
				return err
			}
			return tx.Migrator().DropTable(&nodeInMemoryModelForM20261013{}, &modelFileForM20261013{})
		},
	}})
}

// clearTasksForM20261013 removes every task and releases the nodes that
// reference one as their current task.
func clearTasksForM20261013(tx *gorm.DB) error {
	if err := tx.Model(&nodeTaskRefForM20261013{}).
		Where("current_task_id_commitment IS NOT NULL").
		Update("current_task_id_commitment", nil).Error; err != nil {
		return err
	}
	if err := tx.Model(&nodeTaskRefForM20261013{}).
		Where("status = ?", nodeStatusBusyForM20261013).
		Update("status", nodeStatusAvailableForM20261013).Error; err != nil {
		return err
	}
	return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Delete(&inferenceTaskRowsForM20261013{}).Error
}

func replaceTableForM20261013(tx *gorm.DB, from, to interface{}) error {
	if err := tx.Migrator().DropTable(from); err != nil {
		return err
	}
	return tx.Migrator().CreateTable(to)
}

func migrateCalibrationKeyForM20261013(tx *gorm.DB) error {
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Delete(&gpuExecutionCalibrationHashesForM20261013{}).Error; err != nil {
		return err
	}
	if err := tx.Migrator().DropIndex(&gpuExecutionCalibrationLegacyKeyForM20261013{}, "idx_gpu_execution_calibration_config"); err != nil {
		return err
	}
	for _, column := range calibrationHashColumnsForM20261013 {
		if err := tx.Migrator().AddColumn(&gpuExecutionCalibrationHashesForM20261013{}, column); err != nil {
			return err
		}
	}
	for _, column := range calibrationHashColumnsForM20261013 {
		if err := tx.Migrator().AlterColumn(&gpuExecutionCalibrationRequiredHashesForM20261013{}, column); err != nil {
			return err
		}
	}
	return tx.Migrator().CreateIndex(&gpuExecutionCalibrationKeyForM20261013{}, "idx_gpu_execution_calibration_config")
}

func rollbackCalibrationKeyForM20261013(tx *gorm.DB) error {
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Delete(&gpuExecutionCalibrationHashesForM20261013{}).Error; err != nil {
		return err
	}
	if err := tx.Migrator().DropIndex(&gpuExecutionCalibrationKeyForM20261013{}, "idx_gpu_execution_calibration_config"); err != nil {
		return err
	}
	for _, column := range calibrationHashColumnsForM20261013 {
		if err := tx.Migrator().DropColumn(&gpuExecutionCalibrationHashesForM20261013{}, column); err != nil {
			return err
		}
	}
	return tx.Migrator().CreateIndex(&gpuExecutionCalibrationLegacyKeyForM20261013{}, "idx_gpu_execution_calibration_config")
}
