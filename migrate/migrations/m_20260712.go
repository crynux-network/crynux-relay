package migrations

import (
	"database/sql"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskDeliveredTimeMigration struct {
	DeliveredTime sql.NullTime `gorm:"column:delivered_time;null;default:null"`
}

func (inferenceTaskDeliveredTimeMigration) TableName() string {
	return "inference_tasks"
}

type nodeLastSeenTimeMigration struct {
	LastSeenTime sql.NullTime `gorm:"column:last_seen_time;null;default:null"`
}

func (nodeLastSeenTimeMigration) TableName() string {
	return "nodes"
}

func M20260712(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260712",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&inferenceTaskDeliveredTimeMigration{}, "DeliveredTime"); err != nil {
					return err
				}
				return tx.Migrator().AddColumn(&nodeLastSeenTimeMigration{}, "LastSeenTime")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropColumn(&inferenceTaskDeliveredTimeMigration{}, "DeliveredTime"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&nodeLastSeenTimeMigration{}, "LastSeenTime")
			},
		},
	})
}
