package migrations

import (
	"fmt"
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// normalizeGPUNameForM20260714_1 is the GPU name normalization rule frozen at
// the time this migration was written: trim leading/trailing whitespace and
// collapse internal whitespace runs into single spaces.
func normalizeGPUNameForM20260714_1(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

type nodeGPUNameForM20260714_1 struct {
	ID      uint   `gorm:"primarykey"`
	GPUName string `gorm:"column:gpu_name"`
}

func (nodeGPUNameForM20260714_1) TableName() string {
	return "nodes"
}

type nodeNameCountForM20260714_1 struct {
	ID          uint   `gorm:"primarykey"`
	GPUName     string `gorm:"column:gpu_name"`
	GPUVram     uint64 `gorm:"column:gpu_vram"`
	NodeVersion string `gorm:"column:node_version"`
	ActiveCount uint64 `gorm:"column:active_count"`
}

func (nodeNameCountForM20260714_1) TableName() string {
	return "node_name_counts"
}

type nodeNameWhitelistForM20260714_1 struct {
	ID          uint   `gorm:"primarykey"`
	GPUName     string `gorm:"column:gpu_name"`
	GPUVram     uint64 `gorm:"column:gpu_vram"`
	NodeVersion string `gorm:"column:node_version"`
}

func (nodeNameWhitelistForM20260714_1) TableName() string {
	return "node_name_whitelists"
}

type delegatedStakingSnapshotGPUNameForM20260714_1 struct {
	ID      uint   `gorm:"primarykey"`
	GPUName string `gorm:"column:gpu_name"`
}

func (delegatedStakingSnapshotGPUNameForM20260714_1) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func normalizeNodeGPUNamesForM20260714_1(tx *gorm.DB) error {
	var rows []nodeGPUNameForM20260714_1
	return tx.FindInBatches(&rows, 1000, func(batchTx *gorm.DB, _ int) error {
		for _, row := range rows {
			normalized := normalizeGPUNameForM20260714_1(row.GPUName)
			if normalized == row.GPUName {
				continue
			}
			if err := tx.Model(&nodeGPUNameForM20260714_1{}).
				Where("id = ?", row.ID).
				Update("gpu_name", normalized).Error; err != nil {
				return err
			}
		}
		return nil
	}).Error
}

func mergeNodeNameCountsForM20260714_1(tx *gorm.DB) error {
	var rows []nodeNameCountForM20260714_1
	if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}

	type countGroup struct {
		keeper     nodeNameCountForM20260714_1
		totalCount uint64
		duplicates []uint
		gpuName    string
	}
	groups := make(map[string]*countGroup)
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		normalized := normalizeGPUNameForM20260714_1(row.GPUName)
		key := fmt.Sprintf("%s|%d|%s", normalized, row.GPUVram, row.NodeVersion)
		group, ok := groups[key]
		if !ok {
			groups[key] = &countGroup{
				keeper:     row,
				totalCount: row.ActiveCount,
				gpuName:    normalized,
			}
			order = append(order, key)
			continue
		}
		group.totalCount += row.ActiveCount
		group.duplicates = append(group.duplicates, row.ID)
	}

	for _, key := range order {
		group := groups[key]
		if len(group.duplicates) > 0 {
			if err := tx.Where("id IN ?", group.duplicates).
				Delete(&nodeNameCountForM20260714_1{}).Error; err != nil {
				return err
			}
		}
		if group.keeper.GPUName == group.gpuName && group.keeper.ActiveCount == group.totalCount {
			continue
		}
		if err := tx.Model(&nodeNameCountForM20260714_1{}).
			Where("id = ?", group.keeper.ID).
			Updates(map[string]interface{}{
				"gpu_name":     group.gpuName,
				"active_count": group.totalCount,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func dedupeNodeNameWhitelistsForM20260714_1(tx *gorm.DB) error {
	var rows []nodeNameWhitelistForM20260714_1
	if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}

	keepers := make(map[string]nodeNameWhitelistForM20260714_1)
	var duplicates []uint
	for _, row := range rows {
		normalized := normalizeGPUNameForM20260714_1(row.GPUName)
		key := fmt.Sprintf("%s|%d|%s", normalized, row.GPUVram, row.NodeVersion)
		if _, ok := keepers[key]; ok {
			duplicates = append(duplicates, row.ID)
			continue
		}
		keepers[key] = row
	}

	if len(duplicates) > 0 {
		if err := tx.Where("id IN ?", duplicates).
			Delete(&nodeNameWhitelistForM20260714_1{}).Error; err != nil {
			return err
		}
	}
	for _, keeper := range keepers {
		normalized := normalizeGPUNameForM20260714_1(keeper.GPUName)
		if normalized == keeper.GPUName {
			continue
		}
		if err := tx.Model(&nodeNameWhitelistForM20260714_1{}).
			Where("id = ?", keeper.ID).
			Update("gpu_name", normalized).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeSnapshotGPUNamesForM20260714_1(tx *gorm.DB) error {
	var rows []delegatedStakingSnapshotGPUNameForM20260714_1
	return tx.FindInBatches(&rows, 1000, func(batchTx *gorm.DB, _ int) error {
		for _, row := range rows {
			normalized := normalizeGPUNameForM20260714_1(row.GPUName)
			if normalized == row.GPUName {
				continue
			}
			if err := tx.Model(&delegatedStakingSnapshotGPUNameForM20260714_1{}).
				Where("id = ?", row.ID).
				Update("gpu_name", normalized).Error; err != nil {
				return err
			}
		}
		return nil
	}).Error
}

func M20260714_1(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260714_1",
			Migrate: func(tx *gorm.DB) error {
				if err := normalizeNodeGPUNamesForM20260714_1(tx); err != nil {
					return err
				}
				if err := mergeNodeNameCountsForM20260714_1(tx); err != nil {
					return err
				}
				if err := dedupeNodeNameWhitelistsForM20260714_1(tx); err != nil {
					return err
				}
				return normalizeSnapshotGPUNamesForM20260714_1(tx)
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
}
