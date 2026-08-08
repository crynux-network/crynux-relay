package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeHealthExcludedForM20260808 struct {
	HealthExcluded bool `gorm:"not null;default:false"`
}

func (nodeHealthExcludedForM20260808) TableName() string {
	return "nodes"
}

func M20260808(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260808",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AddColumn(&nodeHealthExcludedForM20260808{}, "HealthExcluded")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&nodeHealthExcludedForM20260808{}, "HealthExcluded")
			},
		},
	})
}
