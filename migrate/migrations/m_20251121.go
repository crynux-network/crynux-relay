package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20251121(db *gorm.DB) *gormigrate.Gormigrate {
	type UserStaking struct {
		gorm.Model
		DelegatorAddress string `json:"delegator_address" gorm:"index;type:string;size:191;not null"`
		NodeAddress      string `json:"node_address" gorm:"index;type:string;size:191;not null"`
		Amount           string `json:"amount" gorm:"type:string;size:191;not null"`
		Valid            bool   `json:"valid" gorm:"not null;index"`
		Network          string `json:"network" gorm:"not null;index;type:string;size:191"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20251121",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().RenameColumn(&UserStaking{}, "user_address", "delegator_address"); err != nil {
					return err
				}
				if err := tx.Migrator().RenameTable("user_stakings", "delegations"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("delegations") {
					if err := tx.Migrator().RenameTable("delegations", "user_stakings"); err != nil {
						return err
					}
				}
				if err := tx.Migrator().RenameColumn(&UserStaking{}, "delegator_address", "user_address"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
