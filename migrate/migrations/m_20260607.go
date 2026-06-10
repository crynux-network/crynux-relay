package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260607(db *gorm.DB) *gormigrate.Gormigrate {
	type VestingRecord struct {
		Slashed bool `gorm:"not null;default:false;index"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260607",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&VestingRecord{}, "Slashed"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropColumn(&VestingRecord{}, "Slashed"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
