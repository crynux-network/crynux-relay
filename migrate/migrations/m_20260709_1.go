package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
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

func M20260709_1(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260709_1",
			Migrate: func(tx *gorm.DB) error {
				// The table may already exist on deployments where a previous
				// failed run created it without recording the migration.
				if tx.Migrator().HasTable(&loadedModelMigration{}) {
					return nil
				}
				return tx.Migrator().CreateTable(&loadedModelMigration{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&loadedModelMigration{})
			},
		},
	})
}
