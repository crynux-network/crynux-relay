package models

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LoadedModelType string

const (
	LoadedModelTypeSD  LoadedModelType = "sd"
	LoadedModelTypeLLM LoadedModelType = "llm"
)

func LoadedModelTypeFromTaskType(taskType TaskType) LoadedModelType {
	if taskType == TaskTypeLLM {
		return LoadedModelTypeLLM
	}
	return LoadedModelTypeSD
}

// LoadedModel is the successful-execution projection of one loaded model. A row
// is identified by the exact model file set, the requested model cache hash and
// the execution dtype of the successful execution.
type LoadedModel struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// RequiredModelFilesHash identifies the exact model file set of the loaded model.
	RequiredModelFilesHash []byte `json:"-" gorm:"column:required_model_files_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:1"`
	// ModelCacheHash identifies the in-memory model cache entry of the loaded model.
	ModelCacheHash []byte `json:"-" gorm:"column:model_cache_hash;type:binary(32);not null;uniqueIndex:idx_loaded_models_identity,priority:2"`
	ExecutionDType string `json:"execution_dtype" gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_loaded_models_identity,priority:3"`
	// RequiredModelFileHashes is the sorted concatenation of the exact member hashes.
	RequiredModelFileHashes []byte          `json:"-" gorm:"type:longblob;not null"`
	QuantizationConfig      JSONText        `json:"quantization_config" gorm:"type:json;not null"`
	ModelID                 string          `json:"model_id" gorm:"not null;size:191"`
	Variant                 string          `json:"variant" gorm:"not null;size:64;default:''"`
	ModelType               LoadedModelType `json:"model_type" gorm:"not null;size:16"`
	MinVRAM                 uint64          `json:"min_vram" gorm:"column:min_vram;not null;index"`
}

// MemberHashes decodes the exact member hashes of the loaded model.
func (m *LoadedModel) MemberHashes() ([]Hash32, error) {
	return DecodeHashList(m.RequiredModelFileHashes)
}
func ListLoadedModels(ctx context.Context, db *gorm.DB) ([]LoadedModel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var loadedModels []LoadedModel
	if err := db.WithContext(dbCtx).Model(&LoadedModel{}).Order("model_id ASC, variant ASC, execution_dtype ASC, id ASC").Find(&loadedModels).Error; err != nil {
		return nil, err
	}
	return loadedModels, nil
}

func UpsertLoadedModelMinVRAMs(ctx context.Context, db *gorm.DB, loadedModels []LoadedModel) error {
	if len(loadedModels) == 0 {
		return nil
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	minVRAMExpr := gorm.Expr("LEAST(min_vram, VALUES(min_vram))")
	updatedAtExpr := gorm.Expr("VALUES(updated_at)")
	if db.Dialector.Name() == "sqlite" {
		minVRAMExpr = gorm.Expr("MIN(min_vram, excluded.min_vram)")
		updatedAtExpr = gorm.Expr("excluded.updated_at")
	}

	return db.WithContext(dbCtx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "required_model_files_hash"}, {Name: "model_cache_hash"}, {Name: "execution_dtype"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"min_vram":   minVRAMExpr,
			"updated_at": updatedAtExpr,
		}),
	}).Create(&loadedModels).Error
}
