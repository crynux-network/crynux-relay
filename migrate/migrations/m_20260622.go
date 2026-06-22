package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260622(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260622",
			Migrate: func(tx *gorm.DB) error {
				return tx.Table("node_models").
					Where("deleted_at IS NULL").
					Update("model_id", gorm.Expr("LOWER(model_id)")).
					Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
}
