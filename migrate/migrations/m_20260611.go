package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260611(db *gorm.DB) *gormigrate.Gormigrate {
	type Delegation struct {
		Valid   bool `gorm:"not null;index"`
		Slashed bool `gorm:"not null;default:false;index"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260611",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&Delegation{}, "Slashed"); err != nil {
					return err
				}
				if err := m.DropColumn(&Delegation{}, "Valid"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&Delegation{}, "Valid"); err != nil {
					return err
				}
				if err := m.DropColumn(&Delegation{}, "Slashed"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
