package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20251106(db *gorm.DB) *gormigrate.Gormigrate {
	type Node struct {
		DelegatorShare uint8 `json:"delegator_share"`
	}

	type UserStaking struct {
		gorm.Model
		UserAddress string `json:"user_address" gorm:"index;type:string;size:191;not null"`
		NodeAddress string `json:"node_address" gorm:"index;type:string;size:191;not null"`
		Amount      string `json:"amount" gorm:"type:string;size:191;not null"`
		Valid       bool   `json:"valid" gorm:"not null;index"`
		Network     string `json:"network" gorm:"not null;index;type:string;size:191"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20251106",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().CreateTable(&UserStaking{}); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&Node{}, "DelegatorShare"); err != nil {
					return err
				}
				if err := tx.Migrator().RenameTable("native_token_listeners", "block_listeners"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("user_stakings") {
					if err := tx.Migrator().DropTable("user_stakings"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasColumn(&Node{}, "DelegatorShare") {
					if err := tx.Migrator().DropColumn(&Node{}, "DelegatorShare"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable("block_listeners") {
					if err := tx.Migrator().RenameTable("block_listeners", "native_token_listeners"); err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
}
