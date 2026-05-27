package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260527_1(db *gorm.DB) *gormigrate.Gormigrate {
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
			ID: "M20260527_1",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.CreateTable(&NodeNameWhitelist{}); err != nil {
					return err
				}
				if err := m.CreateTable(&NodeNameCount{}); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropTable("node_name_counts"); err != nil {
					return err
				}
				if err := m.DropTable("node_name_whitelists"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
