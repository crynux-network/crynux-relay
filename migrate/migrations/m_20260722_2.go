package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type networkNodeDataNetworkForM20260722_2 struct {
	Network string `gorm:"type:string;size:64;not null;default:'';index"`
}

func (networkNodeDataNetworkForM20260722_2) TableName() string {
	return "network_node_data"
}

func M20260722_2(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260722_2",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AddColumn(&networkNodeDataNetworkForM20260722_2{}, "Network")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&networkNodeDataNetworkForM20260722_2{}, "Network")
			},
		},
	})
}
