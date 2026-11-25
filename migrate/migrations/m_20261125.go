package migrations

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type loadedModelIdentityOldForM20261125 struct {
	ID                     uint   `gorm:"primaryKey"`
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:1"`
	ModelCacheHash         []byte `gorm:"column:model_cache_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:2"`
	ExecutionDType         string `gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_loaded_models_identity,priority:3"`
	ModelCacheConfig       string `gorm:"column:model_cache_config;type:longtext;not null"`
	MinVRAM                uint64 `gorm:"column:min_vram;not null"`
}

func (loadedModelIdentityOldForM20261125) TableName() string { return "loaded_models" }

type loadedModelIdentityNewForM20261125 struct {
	RequiredModelFilesHash []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:1"`
	ModelCacheHash         []byte `gorm:"column:model_cache_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:2"`
	DType                  string `gorm:"column:dtype;size:64;not null"`
}

func (loadedModelIdentityNewForM20261125) TableName() string { return "loaded_models" }

type loadedModelRenameDTypeForM20261125 struct {
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null"`
}

func (loadedModelRenameDTypeForM20261125) TableName() string { return "loaded_models" }

type loadedModelDTypeColumnForM20261125 struct {
	DType string `gorm:"column:dtype;size:64;not null"`
}

func (loadedModelDTypeColumnForM20261125) TableName() string { return "loaded_models" }

type nodeInMemoryModelExecutionDTypeForM20261125 struct {
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null;default:''"`
}

func (nodeInMemoryModelExecutionDTypeForM20261125) TableName() string {
	return "node_in_memory_models"
}

type gpuExecutionCalibrationDTypeOldForM20261125 struct {
	ID             uint   `gorm:"primaryKey"`
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null"`
}

func (gpuExecutionCalibrationDTypeOldForM20261125) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationRenameDTypeForM20261125 struct {
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null"`
}

func (gpuExecutionCalibrationRenameDTypeForM20261125) TableName() string {
	return "gpu_execution_calibrations"
}

type gpuExecutionCalibrationDTypeColumnForM20261125 struct {
	DType string `gorm:"column:dtype;size:64;not null"`
}

func (gpuExecutionCalibrationDTypeColumnForM20261125) TableName() string {
	return "gpu_execution_calibrations"
}

type inferenceTaskExecutionDTypeForM20261125 struct {
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null;default:''"`
}

func (inferenceTaskExecutionDTypeForM20261125) TableName() string { return "inference_tasks" }

type inferenceTaskRequestedDTypeForM20261125 struct {
	RequestedDType string `gorm:"column:requested_dtype;size:64;not null;default:'auto'"`
}

func (inferenceTaskRequestedDTypeForM20261125) TableName() string { return "inference_tasks" }

type inferenceTaskDTypeForM20261125 struct {
	DType string `gorm:"column:dtype;size:64;not null;default:''"`
}

func (inferenceTaskDTypeForM20261125) TableName() string { return "inference_tasks" }

// Prerequisite table shapes for SQLite unit tests of this migration.
type loadedModelForM20261125Test struct {
	ID                      uint `gorm:"primaryKey"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	RequiredModelFilesHash  []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:1"`
	ModelCacheHash          []byte `gorm:"column:model_cache_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:2"`
	ExecutionDType          string `gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_loaded_models_identity,priority:3"`
	RequiredModelFileHashes []byte `gorm:"type:longblob;not null"`
	QuantizationConfig      string `gorm:"type:json;not null"`
	ModelID                 string `gorm:"not null;size:191"`
	ModelCacheConfig        string `gorm:"column:model_cache_config;type:longtext;not null"`
	ModelType               string `gorm:"not null;size:16"`
	MinVRAM                 uint64 `gorm:"column:min_vram;not null;index"`
}

func (loadedModelForM20261125Test) TableName() string { return "loaded_models" }

type nodeInMemoryModelForM20261125Test struct {
	NodeAddress             string `gorm:"size:191;primaryKey"`
	CurrentModelCacheHash   []byte `gorm:"type:binary(32);not null"`
	CurrentModelFileHashes  []byte `gorm:"type:longblob;not null"`
	CurrentModelCacheConfig string `gorm:"column:current_model_cache_config;type:longtext;not null"`
	ExecutionDType          string `gorm:"column:execution_dtype;size:64;not null"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (nodeInMemoryModelForM20261125Test) TableName() string { return "node_in_memory_models" }

type gpuExecutionCalibrationForM20261125Test struct {
	ID                       uint   `gorm:"primaryKey"`
	TaskType                 uint8  `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName                  string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram                  uint64 `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName                string `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ExecutionDType           string `gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	RequiredModelFilesHash   []byte `gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	QuantizationConfigHash   []byte `gorm:"column:quantization_config_hash;type:binary(32);not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
	MinVRAMRequirement       uint64 `gorm:"column:min_vram_requirement;not null;default:0"`
	MaxVRAMRequirement       uint64 `gorm:"column:max_vram_requirement;not null;default:0"`
	SecondsPerSDPixelStep    float64
	SDOverheadSeconds        float64
	SDFormulaVersion         uint64
	LLMConstantSeconds       float64
	LLMSecondsPerInputByte   float64
	LLMSecondsPerOutputToken float64
	LLMModelSwitchSeconds    float64
	LLMSecondsPerImage       float64
	LLMSecondsPerMegapixel   float64
	LLMFormulaVersion        uint64
	LLMSuccessSamples        uint64
	SDSuccessSamples         uint64
}

func (gpuExecutionCalibrationForM20261125Test) TableName() string {
	return "gpu_execution_calibrations"
}

type inferenceTaskForM20261125Test struct {
	ID             uint   `gorm:"primaryKey"`
	RequestedDType string `gorm:"column:requested_dtype;size:64;not null;default:'auto'"`
	ExecutionDType string `gorm:"column:execution_dtype;size:64;not null;default:''"`
}

func (inferenceTaskForM20261125Test) TableName() string { return "inference_tasks" }

func isConcreteDTypeForM20261125(dtype string) bool {
	switch strings.ToLower(strings.TrimSpace(dtype)) {
	case "float16", "bfloat16", "float32":
		return true
	default:
		return false
	}
}

func cacheConfigDTypeForM20261125(raw string) string {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var config map[string]any
	if err := decoder.Decode(&config); err != nil {
		return ""
	}
	dtype, _ := config["dtype"].(string)
	return dtype
}

func preferLoadedModelForM20261125(candidate, current loadedModelIdentityOldForM20261125, configDType string) bool {
	candidateMatch := strings.EqualFold(candidate.ExecutionDType, configDType)
	currentMatch := strings.EqualFold(current.ExecutionDType, configDType)
	if candidateMatch != currentMatch {
		return candidateMatch
	}
	if candidate.MinVRAM != current.MinVRAM {
		return candidate.MinVRAM < current.MinVRAM
	}
	return candidate.ID < current.ID
}

func pruneLoadedModelsForM20261125(tx *gorm.DB) error {
	var rows []loadedModelIdentityOldForM20261125
	if err := tx.Model(&loadedModelIdentityOldForM20261125{}).
		Select("id", "required_model_files_hash", "model_cache_hash", "execution_dtype", "model_cache_config", "min_vram").
		Find(&rows).Error; err != nil {
		return err
	}

	deleteIDs := make([]uint, 0)
	bestByKey := make(map[[64]byte]loadedModelIdentityOldForM20261125)
	for _, row := range rows {
		configDType := cacheConfigDTypeForM20261125(row.ModelCacheConfig)
		if !isConcreteDTypeForM20261125(configDType) {
			deleteIDs = append(deleteIDs, row.ID)
			continue
		}
		if len(row.RequiredModelFilesHash) != sha256.Size || len(row.ModelCacheHash) != sha256.Size {
			deleteIDs = append(deleteIDs, row.ID)
			continue
		}
		var key [64]byte
		copy(key[:32], row.RequiredModelFilesHash)
		copy(key[32:], row.ModelCacheHash)
		current, ok := bestByKey[key]
		if !ok {
			bestByKey[key] = row
			continue
		}
		winner, loser := current, row
		if preferLoadedModelForM20261125(row, current, configDType) {
			winner, loser = row, current
		}
		if loser.MinVRAM < winner.MinVRAM {
			winner.MinVRAM = loser.MinVRAM
		}
		bestByKey[key] = winner
		deleteIDs = append(deleteIDs, loser.ID)
	}
	for _, winner := range bestByKey {
		if err := tx.Model(&loadedModelIdentityOldForM20261125{}).
			Where("id = ?", winner.ID).
			Update("min_vram", winner.MinVRAM).Error; err != nil {
			return err
		}
	}
	if len(deleteIDs) == 0 {
		return nil
	}
	return tx.Where("id IN ?", deleteIDs).Delete(&loadedModelIdentityOldForM20261125{}).Error
}

func pruneGPUExecutionCalibrationsForM20261125(tx *gorm.DB) error {
	var rows []gpuExecutionCalibrationDTypeOldForM20261125
	if err := tx.Model(&gpuExecutionCalibrationDTypeOldForM20261125{}).
		Select("id", "execution_dtype").
		Find(&rows).Error; err != nil {
		return err
	}
	deleteIDs := make([]uint, 0)
	for _, row := range rows {
		if !isConcreteDTypeForM20261125(row.ExecutionDType) {
			deleteIDs = append(deleteIDs, row.ID)
		}
	}
	if len(deleteIDs) == 0 {
		return nil
	}
	return tx.Where("id IN ?", deleteIDs).Delete(&gpuExecutionCalibrationDTypeOldForM20261125{}).Error
}

// M20261125 forces concrete dtype identity: renames execution/requested dtype
// columns, drops node in-memory execution_dtype, and simplifies loaded_models
// uniqueness to (required_model_files_hash, model_cache_hash).
func M20261125(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261125",
		Migrate: func(tx *gorm.DB) error {
			if err := pruneLoadedModelsForM20261125(tx); err != nil {
				return err
			}
			if err := tx.Migrator().DropIndex(&loadedModelIdentityOldForM20261125{}, "idx_loaded_models_identity"); err != nil {
				return err
			}
			if err := tx.Migrator().RenameColumn(&loadedModelRenameDTypeForM20261125{}, "execution_dtype", "dtype"); err != nil {
				return err
			}
			if err := tx.Migrator().CreateIndex(&loadedModelIdentityNewForM20261125{}, "idx_loaded_models_identity"); err != nil {
				return err
			}

			if err := tx.Migrator().DropColumn(&nodeInMemoryModelExecutionDTypeForM20261125{}, "ExecutionDType"); err != nil {
				return err
			}

			if err := pruneGPUExecutionCalibrationsForM20261125(tx); err != nil {
				return err
			}
			if err := tx.Migrator().RenameColumn(&gpuExecutionCalibrationRenameDTypeForM20261125{}, "execution_dtype", "dtype"); err != nil {
				return err
			}

			if err := tx.Migrator().DropColumn(&inferenceTaskExecutionDTypeForM20261125{}, "ExecutionDType"); err != nil {
				return err
			}
			if err := tx.Migrator().RenameColumn(&inferenceTaskRequestedDTypeForM20261125{}, "requested_dtype", "dtype"); err != nil {
				return err
			}
			return tx.Migrator().AlterColumn(&inferenceTaskDTypeForM20261125{}, "DType")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().RenameColumn(&inferenceTaskDTypeForM20261125{}, "dtype", "requested_dtype"); err != nil {
				return err
			}
			if err := tx.Migrator().AlterColumn(&inferenceTaskRequestedDTypeForM20261125{}, "RequestedDType"); err != nil {
				return err
			}
			if err := tx.Migrator().AddColumn(&inferenceTaskExecutionDTypeForM20261125{}, "ExecutionDType"); err != nil {
				return err
			}

			if err := tx.Migrator().RenameColumn(&gpuExecutionCalibrationDTypeColumnForM20261125{}, "dtype", "execution_dtype"); err != nil {
				return err
			}

			if err := tx.Migrator().AddColumn(&nodeInMemoryModelExecutionDTypeForM20261125{}, "ExecutionDType"); err != nil {
				return err
			}

			if err := tx.Migrator().DropIndex(&loadedModelIdentityNewForM20261125{}, "idx_loaded_models_identity"); err != nil {
				return err
			}
			if err := tx.Migrator().RenameColumn(&loadedModelDTypeColumnForM20261125{}, "dtype", "execution_dtype"); err != nil {
				return err
			}
			return tx.Migrator().CreateIndex(&loadedModelIdentityOldForM20261125{}, "idx_loaded_models_identity")
		},
	}})
}
