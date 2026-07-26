package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeTaskErrorGpuFieldsForM20260726 struct {
	GpuCount     int    `gorm:"not null;default:0"`
	GpuModel     string `gorm:"type:string;size:255;not null;default:''"`
	GpuVramMb    int64  `gorm:"not null;default:0"`
	ExecutorMode string `gorm:"type:string;size:32;not null;default:''"`
}

func (nodeTaskErrorGpuFieldsForM20260726) TableName() string {
	return "node_task_errors"
}

func M20260726(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260726",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "GpuCount"); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "GpuModel"); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "GpuVramMb"); err != nil {
					return err
				}
				return tx.Migrator().AddColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "ExecutorMode")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "ExecutorMode"); err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "GpuVramMb"); err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "GpuModel"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&nodeTaskErrorGpuFieldsForM20260726{}, "GpuCount")
			},
		},
	})
}
