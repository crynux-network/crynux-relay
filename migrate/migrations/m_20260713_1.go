package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeModelSoftDeleteCleanupMigration struct {
	ID        uint           `gorm:"primaryKey"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (nodeModelSoftDeleteCleanupMigration) TableName() string {
	return "node_models"
}

// M20260713_1 converts node_models to hard-delete semantics and repairs
// hf_model_id casing.
//
// The original M20260713 backfill matched rows by model_id value. Under the
// production MySQL case-insensitive collation, a case-variant model_id kept on
// soft-deleted historical rows could be picked as the update source and
// overwrite the hf_model_id of live lowercase rows with a mixed-case value,
// which then failed the case-sensitive match against loaded_models.model_id.
//
// Soft-deleted rows carry no application state, so they are removed for good
// and the deleted_at column is dropped, eliminating the stale case-variant
// model IDs entirely.
func M20260713_1(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260713_1",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Unscoped().
					Where("deleted_at IS NOT NULL").
					Delete(&nodeModelSoftDeleteCleanupMigration{}).Error; err != nil {
					return err
				}
				if err := tx.Table("node_models").
					Where("hf_model_id <> ''").
					Update("hf_model_id", gorm.Expr("LOWER(hf_model_id)")).Error; err != nil {
					return err
				}
				if err := tx.Migrator().DropIndex(&nodeModelSoftDeleteCleanupMigration{}, "DeletedAt"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&nodeModelSoftDeleteCleanupMigration{}, "DeletedAt")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&nodeModelSoftDeleteCleanupMigration{}, "DeletedAt"); err != nil {
					return err
				}
				return tx.Migrator().CreateIndex(&nodeModelSoftDeleteCleanupMigration{}, "DeletedAt")
			},
		},
	})
}
