package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260604(db *gorm.DB) *gormigrate.Gormigrate {
	type TaskWhitelist struct {
		gorm.Model
		Address string `gorm:"not null;size:42;uniqueIndex:idx_task_whitelist_address"`
	}

	type NodeNameWhitelist struct {
		gorm.Model
		GPUName     string `gorm:"not null;size:191;uniqueIndex:idx_node_name_whitelist_unique"`
		GPUVram     uint64 `gorm:"not null;uniqueIndex:idx_node_name_whitelist_unique"`
		NodeVersion string `gorm:"not null;size:32;uniqueIndex:idx_node_name_whitelist_unique"`
	}

	type NodeNameCount struct {
		gorm.Model
		GPUName     string `gorm:"not null;size:191;uniqueIndex:idx_node_name_count_unique"`
		GPUVram     uint64 `gorm:"not null;uniqueIndex:idx_node_name_count_unique"`
		NodeVersion string `gorm:"not null;size:32;uniqueIndex:idx_node_name_count_unique"`
		ActiveCount uint64 `gorm:"not null"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260604",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Unscoped().Where("deleted_at IS NOT NULL").Delete(&TaskWhitelist{}).Error; err != nil {
					return err
				}
				if err := tx.Unscoped().Where("deleted_at IS NOT NULL").Delete(&NodeNameWhitelist{}).Error; err != nil {
					return err
				}
				if err := tx.Unscoped().Where("active_count = ?", 0).Delete(&NodeNameCount{}).Error; err != nil {
					return err
				}

				m := tx.Migrator()
				if err := m.DropColumn(&TaskWhitelist{}, "DeletedAt"); err != nil {
					return err
				}
				if err := m.DropColumn(&NodeNameWhitelist{}, "DeletedAt"); err != nil {
					return err
				}
				if err := m.DropColumn(&NodeNameCount{}, "DeletedAt"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&TaskWhitelist{}, "DeletedAt"); err != nil {
					return err
				}
				if err := m.AddColumn(&NodeNameWhitelist{}, "DeletedAt"); err != nil {
					return err
				}
				if err := m.AddColumn(&NodeNameCount{}, "DeletedAt"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
