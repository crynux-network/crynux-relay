package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskPricingMigration struct {
	ID                   uint    `gorm:"primaryKey;index:idx_inference_tasks_status_priority_id,priority:3"`
	Status               uint8   `gorm:"index:idx_inference_tasks_status_priority_id,priority:1"`
	Priority             string  `gorm:"column:priority;type:decimal(65,0);not null;default:0;index:idx_inference_tasks_status_priority_id,priority:2,sort:desc"`
	EstimatedNodeSeconds float64 `gorm:"column:estimated_node_seconds;not null;default:0"`
	VRAMWeight           float64 `gorm:"column:vram_weight;not null;default:0"`
	PricingUnits         float64 `gorm:"column:pricing_units;not null;default:0"`
}

func (inferenceTaskPricingMigration) TableName() string {
	return "inference_tasks"
}

func M20260714(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260714",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&inferenceTaskPricingMigration{}, "Priority"); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&inferenceTaskPricingMigration{}, "EstimatedNodeSeconds"); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&inferenceTaskPricingMigration{}, "VRAMWeight"); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&inferenceTaskPricingMigration{}, "PricingUnits"); err != nil {
					return err
				}
				return tx.Migrator().CreateIndex(&inferenceTaskPricingMigration{}, "idx_inference_tasks_status_priority_id")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropIndex(&inferenceTaskPricingMigration{}, "idx_inference_tasks_status_priority_id"); err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&inferenceTaskPricingMigration{}, "PricingUnits"); err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&inferenceTaskPricingMigration{}, "VRAMWeight"); err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&inferenceTaskPricingMigration{}, "EstimatedNodeSeconds"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&inferenceTaskPricingMigration{}, "Priority")
			},
		},
	})
}
