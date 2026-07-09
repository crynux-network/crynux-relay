package models

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LoadedModel struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ModelID   string    `json:"model_id" gorm:"not null;size:191;uniqueIndex"`
	MinVRAM   uint64    `json:"min_vram" gorm:"column:min_vram;not null;index"`
}

func ListLoadedModels(ctx context.Context, db *gorm.DB) ([]LoadedModel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var loadedModels []LoadedModel
	if err := db.WithContext(dbCtx).Model(&LoadedModel{}).Order("model_id ASC").Find(&loadedModels).Error; err != nil {
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
		Columns: []clause.Column{{Name: "model_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"min_vram":   minVRAMExpr,
			"updated_at": updatedAtExpr,
		}),
	}).Create(&loadedModels).Error
}
