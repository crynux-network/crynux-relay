package migrations

import (
	"strings"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const (
	loadedModelTaskEndSuccess      int = 8
	loadedModelTaskEndGroupRefund  int = 10
	loadedModelTaskEndGroupSuccess int = 11
)

type loadedModelMigration struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ModelID   string `gorm:"not null;size:191;uniqueIndex"`
	MinVRAM   uint64 `gorm:"column:min_vram;not null;index"`
}

func (loadedModelMigration) TableName() string {
	return "loaded_models"
}

type loadedModelBackfillTaskRow struct {
	ModelIDs string
	GPUVram  uint64
}

func backfillLoadedModels(tx *gorm.DB) error {
	modelMinVRAM := make(map[string]uint64)
	var rows []loadedModelBackfillTaskRow

	err := tx.Table("inference_tasks").
		Select("inference_tasks.model_ids, nodes.gpu_vram").
		Joins("JOIN nodes ON nodes.address = inference_tasks.selected_node").
		Where("inference_tasks.status IN ?", []int{
			loadedModelTaskEndSuccess,
			loadedModelTaskEndGroupRefund,
			loadedModelTaskEndGroupSuccess,
		}).
		Where("inference_tasks.selected_node <> ''").
		FindInBatches(&rows, 1000, func(tx *gorm.DB, batch int) error {
			for _, row := range rows {
				for _, modelID := range strings.Split(row.ModelIDs, ";") {
					if modelID == "" {
						continue
					}
					if currentMin, ok := modelMinVRAM[modelID]; !ok || row.GPUVram < currentMin {
						modelMinVRAM[modelID] = row.GPUVram
					}
				}
			}
			return nil
		}).Error
	if err != nil {
		return err
	}
	if len(modelMinVRAM) == 0 {
		return nil
	}

	now := time.Now().UTC()
	loadedModels := make([]loadedModelMigration, 0, len(modelMinVRAM))
	for modelID, minVRAM := range modelMinVRAM {
		loadedModels = append(loadedModels, loadedModelMigration{
			CreatedAt: now,
			UpdatedAt: now,
			ModelID:   modelID,
			MinVRAM:   minVRAM,
		})
	}
	return tx.Create(&loadedModels).Error
}

func M20260709_1(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260709_1",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().CreateTable(&loadedModelMigration{}); err != nil {
					return err
				}
				return backfillLoadedModels(tx)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&loadedModelMigration{})
			},
		},
	})
}
