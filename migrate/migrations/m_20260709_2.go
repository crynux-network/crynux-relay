package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type loadedModelModelTypeMigration struct {
	ModelType string `gorm:"column:model_type;not null;size:16"`
}

func (loadedModelModelTypeMigration) TableName() string {
	return "loaded_models"
}

func M20260709_2(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260709_2",
			Migrate: func(tx *gorm.DB) error {
				// Existing rows store internal model dispatch IDs and carry no
				// task type information, so the projection is cleared and
				// rebuilt from subsequent successful task executions.
				if err := tx.Exec("DELETE FROM loaded_models").Error; err != nil {
					return err
				}
				return tx.Migrator().AddColumn(&loadedModelModelTypeMigration{}, "ModelType")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&loadedModelModelTypeMigration{}, "ModelType")
			},
		},
	})
}
