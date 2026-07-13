package migrations

import (
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeModelHFModelIDMigration struct {
	ID          uint   `gorm:"primaryKey"`
	NodeAddress string `gorm:"size:191;index:idx_node_models_hf_model_id_node_address,priority:2"`
	ModelID     string `gorm:"size:191"`
	HFModelID   string `gorm:"column:hf_model_id;not null;default:'';size:191;index:idx_node_models_hf_model_id_node_address,priority:1"`
}

func (nodeModelHFModelIDMigration) TableName() string {
	return "node_models"
}

// hfModelIDForMigration mirrors models.BaseModelHuggingFaceID, frozen at the
// time this migration was written.
func hfModelIDForMigration(modelID string) string {
	name, ok := strings.CutPrefix(modelID, "base:")
	if !ok {
		return ""
	}
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return ""
	}
	if variantSep := strings.IndexByte(name, '+'); variantSep >= 0 {
		name = name[:variantSep]
	}
	return name
}

func M20260713(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260713",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&nodeModelHFModelIDMigration{}, "HFModelID"); err != nil {
					return err
				}
				if err := tx.Migrator().CreateIndex(&nodeModelHFModelIDMigration{}, "idx_node_models_hf_model_id_node_address"); err != nil {
					return err
				}

				var modelIDs []string
				if err := tx.Table("node_models").
					Distinct("model_id").
					Pluck("model_id", &modelIDs).Error; err != nil {
					return err
				}
				for _, modelID := range modelIDs {
					hfModelID := hfModelIDForMigration(modelID)
					if hfModelID == "" {
						continue
					}
					if err := tx.Table("node_models").
						Where("model_id = ?", modelID).
						Update("hf_model_id", hfModelID).Error; err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropIndex(&nodeModelHFModelIDMigration{}, "idx_node_models_hf_model_id_node_address"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&nodeModelHFModelIDMigration{}, "HFModelID")
			},
		},
	})
}
