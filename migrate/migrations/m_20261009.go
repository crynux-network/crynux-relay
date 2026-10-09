package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type loadedModelVariantForM20261009 struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ModelID   string `gorm:"column:model_id;not null;size:191;uniqueIndex:idx_loaded_models_model_id"`
	ModelType string `gorm:"column:model_type;not null;size:16"`
	MinVRAM   uint64 `gorm:"column:min_vram;not null;index"`
	Variant   string `gorm:"column:variant;not null;size:64;default:''"`
}

func (loadedModelVariantForM20261009) TableName() string {
	return "loaded_models"
}

type loadedModelCompositeUniqueForM20261009 struct {
	ModelID string `gorm:"column:model_id;not null;size:191;uniqueIndex:idx_loaded_models_model_id_variant,priority:1"`
	Variant string `gorm:"column:variant;not null;size:64;default:'';uniqueIndex:idx_loaded_models_model_id_variant,priority:2"`
}

func (loadedModelCompositeUniqueForM20261009) TableName() string {
	return "loaded_models"
}

type loadedModelOldUniqueForM20261009 struct {
	ModelID string `gorm:"column:model_id;uniqueIndex:idx_loaded_models_model_id"`
}

func (loadedModelOldUniqueForM20261009) TableName() string {
	return "loaded_models"
}

func M20261009(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261009",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().AddColumn(&loadedModelVariantForM20261009{}, "Variant"); err != nil {
				return err
			}
			if err := tx.Exec("UPDATE loaded_models SET variant = ? WHERE model_type = ?", "fp16", "sd").Error; err != nil {
				return err
			}
			if err := dedupeLoadedModelsForM20261009(tx); err != nil {
				return err
			}
			if err := tx.Migrator().DropIndex(&loadedModelOldUniqueForM20261009{}, "idx_loaded_models_model_id"); err != nil {
				return err
			}
			return tx.Migrator().CreateIndex(&loadedModelCompositeUniqueForM20261009{}, "idx_loaded_models_model_id_variant")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropIndex(&loadedModelCompositeUniqueForM20261009{}, "idx_loaded_models_model_id_variant"); err != nil {
				return err
			}
			if err := tx.Migrator().CreateIndex(&loadedModelOldUniqueForM20261009{}, "idx_loaded_models_model_id"); err != nil {
				return err
			}
			return tx.Migrator().DropColumn(&loadedModelVariantForM20261009{}, "Variant")
		},
	}})
}

func dedupeLoadedModelsForM20261009(tx *gorm.DB) error {
	var rows []loadedModelVariantForM20261009
	if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	type key struct {
		ModelID string
		Variant string
	}
	keep := make(map[key]uint, len(rows))
	deleteIDs := make([]uint, 0)
	for _, row := range rows {
		k := key{ModelID: row.ModelID, Variant: row.Variant}
		existingID, ok := keep[k]
		if !ok {
			keep[k] = row.ID
			continue
		}
		var existing loadedModelVariantForM20261009
		if err := tx.First(&existing, existingID).Error; err != nil {
			return err
		}
		if row.MinVRAM < existing.MinVRAM || (row.MinVRAM == existing.MinVRAM && row.ID < existing.ID) {
			deleteIDs = append(deleteIDs, existing.ID)
			keep[k] = row.ID
			continue
		}
		deleteIDs = append(deleteIDs, row.ID)
	}
	if len(deleteIDs) == 0 {
		return nil
	}
	return tx.Where("id IN ?", deleteIDs).Delete(&loadedModelVariantForM20261009{}).Error
}
