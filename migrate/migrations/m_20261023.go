package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type loadedModelJSONConfigForM20261023 struct {
	ModelCacheConfig string `gorm:"column:model_cache_config;type:json;not null"`
}

func (loadedModelJSONConfigForM20261023) TableName() string { return "loaded_models" }

type loadedModelLongtextConfigForM20261023 struct {
	ModelCacheConfig string `gorm:"column:model_cache_config;type:longtext;not null"`
}

func (loadedModelLongtextConfigForM20261023) TableName() string { return "loaded_models" }

type loadedModelRowsForM20261023 struct {
	ID uint `gorm:"primaryKey"`
}

func (loadedModelRowsForM20261023) TableName() string { return "loaded_models" }

type nodeInMemoryModelJSONConfigForM20261023 struct {
	CurrentModelCacheConfig string `gorm:"column:current_model_cache_config;type:json;not null"`
}

func (nodeInMemoryModelJSONConfigForM20261023) TableName() string {
	return "node_in_memory_models"
}

type nodeInMemoryModelLongtextConfigForM20261023 struct {
	CurrentModelCacheConfig string `gorm:"column:current_model_cache_config;type:longtext;not null"`
}

func (nodeInMemoryModelLongtextConfigForM20261023) TableName() string {
	return "node_in_memory_models"
}

// Prerequisite table shapes for SQLite unit tests of this migration.
type loadedModelForM20261023Test struct {
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

func (loadedModelForM20261023Test) TableName() string { return "loaded_models" }

type nodeInMemoryModelForM20261023Test struct {
	NodeAddress             string `gorm:"size:191;primaryKey"`
	CurrentModelCacheHash   []byte `gorm:"type:binary(32);not null"`
	CurrentModelFileHashes  []byte `gorm:"type:longblob;not null"`
	CurrentModelCacheConfig string `gorm:"column:current_model_cache_config;type:json;not null"`
	ExecutionDType          string `gorm:"column:execution_dtype;size:64;not null"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (nodeInMemoryModelForM20261023Test) TableName() string { return "node_in_memory_models" }

// M20261023 stores model cache config as longtext so MySQL preserves the
// canonical JSON bytes used to compute model_cache_hash.
func M20261023(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261023",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
				Delete(&loadedModelRowsForM20261023{}).Error; err != nil {
				return err
			}
			if err := tx.Migrator().AlterColumn(&loadedModelLongtextConfigForM20261023{}, "ModelCacheConfig"); err != nil {
				return err
			}
			return tx.Migrator().AlterColumn(&nodeInMemoryModelLongtextConfigForM20261023{}, "CurrentModelCacheConfig")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
				Delete(&loadedModelRowsForM20261023{}).Error; err != nil {
				return err
			}
			if err := tx.Migrator().AlterColumn(&loadedModelJSONConfigForM20261023{}, "ModelCacheConfig"); err != nil {
				return err
			}
			return tx.Migrator().AlterColumn(&nodeInMemoryModelJSONConfigForM20261023{}, "CurrentModelCacheConfig")
		},
	}})
}
