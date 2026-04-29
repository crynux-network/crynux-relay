package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260429(db *gorm.DB) *gormigrate.Gormigrate {
	type Delegation struct {
		gorm.Model
		DelegatorAddress string    `gorm:"uniqueIndex:idx_delegator_node_network;type:string;size:191;not null"`
		NodeAddress      string    `gorm:"uniqueIndex:idx_delegator_node_network;type:string;size:191;not null"`
		Network          string    `gorm:"uniqueIndex:idx_delegator_node_network;type:string;size:191;not null"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260429",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasIndex(&Delegation{}, "idx_delegator_node_network") {
					if err := tx.Migrator().CreateIndex(&Delegation{}, "idx_delegator_node_network"); err != nil {
						return err
					}
				}

				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasIndex(&Delegation{}, "idx_delegator_node_network") {
					if err := tx.Migrator().DropIndex(&Delegation{}, "idx_delegator_node_network"); err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
}
