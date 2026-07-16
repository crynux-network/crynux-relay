package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeModelDownloadSelectionForM20260716 struct {
	ID          uint `gorm:"primaryKey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ModelID     string `gorm:"size:255;uniqueIndex:idx_node_model_download_selection_active,priority:1"`
	NodeAddress string `gorm:"size:42;index;uniqueIndex:idx_node_model_download_selection_active,priority:2"`
	SentAt      time.Time
	Deadline    time.Time
	Status      string `gorm:"size:16;index"`
	Active      *bool  `gorm:"uniqueIndex:idx_node_model_download_selection_active,priority:3"`
}

func (nodeModelDownloadSelectionForM20260716) TableName() string {
	return "node_model_download_selections"
}

func M20260716(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260716",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().CreateTable(&nodeModelDownloadSelectionForM20260716{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&nodeModelDownloadSelectionForM20260716{})
			},
		},
	})
}
