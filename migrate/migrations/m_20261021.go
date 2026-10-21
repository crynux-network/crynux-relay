package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type modelWhitelistForM20261021 struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ModelName string `gorm:"not null;size:512;uniqueIndex:idx_model_whitelist_model_name"`
}

func (modelWhitelistForM20261021) TableName() string { return "model_whitelists" }

// M20261021 creates the model_whitelists table used by the model name allowlist.
func M20261021(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261021",
		Migrate: func(tx *gorm.DB) error {
			return tx.Migrator().CreateTable(&modelWhitelistForM20261021{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&modelWhitelistForM20261021{})
		},
	}})
}
