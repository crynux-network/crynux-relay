package migrations

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const stakingResetNetworkForM20260812 = "crynux-on-base"

type nodeForM20260812 struct {
	Address                 string
	Network                 string
	CurrentTaskIDCommitment *string
}

func (nodeForM20260812) TableName() string { return "nodes" }

type nodeModelForM20260812 struct{}

func (nodeModelForM20260812) TableName() string { return "node_models" }

type nodeModelDownloadSelectionForM20260812 struct{}

func (nodeModelDownloadSelectionForM20260812) TableName() string {
	return "node_model_download_selections"
}

type nodeNameCountForM20260812 struct{}

func (nodeNameCountForM20260812) TableName() string { return "node_name_counts" }

type delegationForM20260812 struct{}

func (delegationForM20260812) TableName() string { return "delegations" }

type delegatedSlashJobForM20260812 struct{}

func (delegatedSlashJobForM20260812) TableName() string { return "delegated_slash_jobs" }

type delegatedStakingSlashRecordForM20260812 struct{}

func (delegatedStakingSlashRecordForM20260812) TableName() string {
	return "delegated_staking_slash_records"
}

type delegatedStakingNodeListSnapshotForM20260812 struct{}

func (delegatedStakingNodeListSnapshotForM20260812) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

type delegationTaskFeeLeaderboardSnapshotForM20260812 struct{}

func (delegationTaskFeeLeaderboardSnapshotForM20260812) TableName() string {
	return "delegation_task_fee_leaderboard_snapshots"
}

type blockchainCursorForM20260812 struct{}

func (blockchainCursorForM20260812) TableName() string { return "blockchain_cursors" }

func resetStakingStateForM20260812(tx *gorm.DB) error {
	var nonTargetNodes int64
	if err := tx.Model(&nodeForM20260812{}).
		Where("network IS NULL OR network <> ?", stakingResetNetworkForM20260812).
		Count(&nonTargetNodes).Error; err != nil {
		return err
	}
	if nonTargetNodes != 0 {
		return fmt.Errorf("staking reset requires all nodes to belong to %s", stakingResetNetworkForM20260812)
	}

	var nodesWithTasks int64
	if err := tx.Model(&nodeForM20260812{}).
		Where("network = ?", stakingResetNetworkForM20260812).
		Where("current_task_id_commitment IS NOT NULL AND current_task_id_commitment <> ''").
		Count(&nodesWithTasks).Error; err != nil {
		return err
	}
	if nodesWithTasks != 0 {
		return fmt.Errorf("staking reset requires all %s node task commitments to be empty", stakingResetNetworkForM20260812)
	}

	if err := tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&delegatedStakingSlashRecordForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&delegatedSlashJobForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&delegationForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&delegatedStakingNodeListSnapshotForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&delegationTaskFeeLeaderboardSnapshotForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&nodeModelDownloadSelectionForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&nodeModelForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&nodeNameCountForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&nodeForM20260812{}).Error; err != nil {
		return err
	}
	return tx.Unscoped().Where("network = ?", stakingResetNetworkForM20260812).Delete(&blockchainCursorForM20260812{}).Error
}

func M20260812(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260812",
			Migrate: func(tx *gorm.DB) error {
				return tx.Transaction(resetStakingStateForM20260812)
			},
			Rollback: func(*gorm.DB) error {
				return nil
			},
		},
	})
}
