package migrations

import (
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeModelForLowercaseMigration struct {
	ID          uint
	NodeAddress string
	ModelID     string
	InUse       bool
}

func M20260622(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260622",
			Migrate: func(tx *gorm.DB) error {
				var rows []nodeModelForLowercaseMigration
				if err := tx.Table("node_models").
					Select("id", "node_address", "model_id", "in_use").
					Where("deleted_at IS NULL").
					Order("id").
					Find(&rows).Error; err != nil {
					return err
				}

				type normalizedKey struct {
					nodeAddress string
					modelID     string
				}
				type normalizedGroup struct {
					keepID       uint
					normalizedID string
					inUse        bool
					deleteIDs    []uint
				}

				groups := make(map[normalizedKey]*normalizedGroup)
				for _, row := range rows {
					normalizedID := strings.ToLower(row.ModelID)
					key := normalizedKey{nodeAddress: row.NodeAddress, modelID: normalizedID}
					group, ok := groups[key]
					if !ok {
						groups[key] = &normalizedGroup{
							keepID:       row.ID,
							normalizedID: normalizedID,
							inUse:        row.InUse,
						}
						continue
					}
					group.inUse = group.inUse || row.InUse
					group.deleteIDs = append(group.deleteIDs, row.ID)
				}

				for _, group := range groups {
					if err := tx.Table("node_models").
						Where("id = ?", group.keepID).
						Updates(map[string]interface{}{
							"model_id": group.normalizedID,
							"in_use":   group.inUse,
						}).Error; err != nil {
						return err
					}
					if len(group.deleteIDs) > 0 {
						if err := tx.Unscoped().
							Table("node_models").
							Where("id IN ?", group.deleteIDs).
							Delete(&nodeModelForLowercaseMigration{}).Error; err != nil {
							return err
						}
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
}
