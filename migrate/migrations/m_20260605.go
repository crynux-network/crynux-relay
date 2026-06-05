package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260605(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260605",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().RenameTable("block_listeners", "blockchain_cursors")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().RenameTable("blockchain_cursors", "block_listeners")
			},
		},
	})
}
