package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260526(db *gorm.DB) *gormigrate.Gormigrate {
	type VestingRecord struct {
		gorm.Model
		Address        string    `gorm:"not null;index"`
		TotalAmount    string    `gorm:"not null;type:string;size:255"`
		ReleasedAmount string    `gorm:"not null;type:string;size:255"`
		StartTime      time.Time `gorm:"not null;index"`
		DurationDays   uint      `gorm:"not null"`
		Source         string    `gorm:"not null;size:64;uniqueIndex:idx_vesting_source_external_id"`
		ExternalID     string    `gorm:"not null;size:128;uniqueIndex:idx_vesting_source_external_id"`
		AdminSignature string    `gorm:"not null;size:255"`
		Status         int8      `gorm:"not null;default:0;index"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260526",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.CreateTable(&VestingRecord{}); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropTable("vesting_records"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
