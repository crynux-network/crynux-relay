package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type pendingSlash struct {
	gorm.Model
	Status           string `gorm:"not null;size:32;index"`
	NodeAddress      string `gorm:"not null;size:191;index"`
	Network          string `gorm:"not null;size:191;index"`
	TaskIDCommitment string `gorm:"not null;size:191;index"`
	EvidenceJSON     string `gorm:"type:longtext;not null"`
	EvidenceComplete bool   `gorm:"not null;index"`
}

func M20260619(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260619",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().CreateTable(&pendingSlash{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&pendingSlash{})
			},
		},
	})
}
