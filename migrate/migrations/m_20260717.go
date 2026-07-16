package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeModelDownloadSelectionMinVRAMMigration struct {
	MinVRAM uint64 `gorm:"column:min_vram;not null;default:0"`
}

func (nodeModelDownloadSelectionMinVRAMMigration) TableName() string {
	return "node_model_download_selections"
}

func M20260717(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260717",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AddColumn(&nodeModelDownloadSelectionMinVRAMMigration{}, "MinVRAM")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&nodeModelDownloadSelectionMinVRAMMigration{}, "MinVRAM")
			},
		},
	})
}
