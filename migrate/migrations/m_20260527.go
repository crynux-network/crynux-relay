package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260527(db *gorm.DB) *gormigrate.Gormigrate {
	type TaskWhitelist struct {
		gorm.Model
		Address string `gorm:"not null;size:42;uniqueIndex:idx_task_whitelist_address"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260527",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.CreateTable(&TaskWhitelist{}); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropTable("task_whitelists"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
