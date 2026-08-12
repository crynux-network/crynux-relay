package migrations

import (
	"crynux_relay/config"
	"fmt"
	"sort"
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

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

func resetStakingStateForM20260812(tx *gorm.DB, targetNetworks []string) error {
	if len(targetNetworks) == 0 {
		return fmt.Errorf("staking reset requires at least one configured blockchain network")
	}

	var nonTargetNodes int64
	if err := tx.Model(&nodeForM20260812{}).
		Where("network IS NULL OR network NOT IN ?", targetNetworks).
		Count(&nonTargetNodes).Error; err != nil {
		return err
	}
	if nonTargetNodes != 0 {
		return fmt.Errorf(
			"staking reset requires all nodes to belong to configured blockchain networks: %s",
			strings.Join(targetNetworks, ", "),
		)
	}

	var nodesWithTasks int64
	if err := tx.Model(&nodeForM20260812{}).
		Where("network IN ?", targetNetworks).
		Where("current_task_id_commitment IS NOT NULL AND current_task_id_commitment <> ''").
		Count(&nodesWithTasks).Error; err != nil {
		return err
	}
	if nodesWithTasks != 0 {
		return fmt.Errorf(
			"staking reset requires all node task commitments on configured blockchain networks to be empty: %s",
			strings.Join(targetNetworks, ", "),
		)
	}

	if err := tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&delegatedStakingSlashRecordForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&delegatedSlashJobForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&delegationForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&delegatedStakingNodeListSnapshotForM20260812{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&delegationTaskFeeLeaderboardSnapshotForM20260812{}).Error; err != nil {
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
	if err := tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&nodeForM20260812{}).Error; err != nil {
		return err
	}
	return tx.Unscoped().Where("network IN ?", targetNetworks).Delete(&blockchainCursorForM20260812{}).Error
}

func configuredStakingResetNetworksForM20260812() []string {
	conf := config.GetConfig()
	targetNetworks := make([]string, 0, len(conf.Blockchains))
	for network := range conf.Blockchains {
		targetNetworks = append(targetNetworks, network)
	}
	sort.Strings(targetNetworks)
	return targetNetworks
}

func m20260812WithTargetNetworks(db *gorm.DB, targetNetworks []string) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260812",
			Migrate: func(tx *gorm.DB) error {
				return tx.Transaction(func(tx *gorm.DB) error {
					return resetStakingStateForM20260812(tx, targetNetworks)
				})
			},
			Rollback: func(*gorm.DB) error {
				return nil
			},
		},
	})
}

func M20260812(db *gorm.DB) *gormigrate.Gormigrate {
	return m20260812WithTargetNetworks(db, configuredStakingResetNetworksForM20260812())
}
