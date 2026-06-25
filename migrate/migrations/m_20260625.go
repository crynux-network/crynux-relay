package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type networkNodeDataRemoveDeletedAtMigration struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Balance   string         `gorm:"type:string;size:255"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (networkNodeDataRemoveDeletedAtMigration) TableName() string {
	return "network_node_data"
}

func M20260625(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260625",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Unscoped().
					Model(&networkNodeDataRemoveDeletedAtMigration{}).
					Where("deleted_at IS NOT NULL").
					Update("deleted_at", nil).Error; err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&networkNodeDataRemoveDeletedAtMigration{}, "DeletedAt"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&networkNodeDataRemoveDeletedAtMigration{}, "Balance")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&networkNodeDataRemoveDeletedAtMigration{}, "Balance"); err != nil {
					return err
				}
				return tx.Migrator().AddColumn(&networkNodeDataRemoveDeletedAtMigration{}, "DeletedAt")
			},
		},
	})
}
