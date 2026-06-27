package migrations

import (
	"crynux_relay/models"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260627(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260627",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().CreateTable(&models.DelegatedStakingNodeListSnapshot{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&models.DelegatedStakingNodeListSnapshot{})
			},
		},
	})
}
