package service

import (
	"context"
	"crynux_relay/models"
	"time"

	"gorm.io/gorm"
)

const DelegationTaskFeeLeaderboardSize = 10

type delegationTaskFeeLeaderboardRow struct {
	DelegatorAddress string
	NodeAddress      string
	Network          string
	StakingAmount    models.BigInt
	TaskFee          models.BigInt
	DelegationApr12m float64 `gorm:"column:delegation_apr_12m"`
}

// RebuildDelegationTaskFeeLeaderboardSnapshots recalculates the current UTC-day
// top delegations by task fee and replaces the leaderboard snapshot table content.
func RebuildDelegationTaskFeeLeaderboardSnapshots(ctx context.Context, db *gorm.DB, now time.Time) error {
	dbCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	dayStart := now.UTC().Truncate(24 * time.Hour)

	var rows []delegationTaskFeeLeaderboardRow
	if err := db.WithContext(dbCtx).Model(&models.UserStakingEarning{}).
		Select("user_staking_earnings.user_address AS delegator_address, user_staking_earnings.node_address AS node_address, user_staking_earnings.network AS network, delegations.amount AS staking_amount, user_staking_earnings.earning AS task_fee, COALESCE(delegated_staking_node_list_snapshots.delegation_apr_12m, 0) AS delegation_apr_12m").
		Joins("INNER JOIN delegations ON delegations.delegator_address = user_staking_earnings.user_address AND delegations.node_address = user_staking_earnings.node_address AND delegations.network = user_staking_earnings.network AND delegations.slashed = ? AND delegations.deleted_at IS NULL", false).
		Joins("LEFT JOIN delegated_staking_node_list_snapshots ON delegated_staking_node_list_snapshots.node_address = user_staking_earnings.node_address").
		Where("user_staking_earnings.time = ?", dayStart).
		Order("CAST(user_staking_earnings.earning AS DECIMAL(65,0)) DESC").
		Order("user_staking_earnings.user_address ASC").
		Order("user_staking_earnings.node_address ASC").
		Order("user_staking_earnings.network ASC").
		Limit(DelegationTaskFeeLeaderboardSize).
		Find(&rows).Error; err != nil {
		return err
	}

	snapshots := make([]models.DelegationTaskFeeLeaderboardSnapshot, 0, len(rows))
	for i, row := range rows {
		snapshots = append(snapshots, models.DelegationTaskFeeLeaderboardSnapshot{
			Rank:             uint8(i + 1),
			DelegatorAddress: row.DelegatorAddress,
			NodeAddress:      row.NodeAddress,
			Network:          row.Network,
			StakingAmount:    row.StakingAmount,
			TaskFee:          row.TaskFee,
			DelegationApr12m: row.DelegationApr12m,
		})
	}

	return db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.DelegationTaskFeeLeaderboardSnapshot{}).Error; err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return nil
		}
		return tx.Create(&snapshots).Error
	})
}

func GetDelegationTaskFeeLeaderboard(ctx context.Context, db *gorm.DB) ([]models.DelegationTaskFeeLeaderboardSnapshot, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var snapshots []models.DelegationTaskFeeLeaderboardSnapshot
	if err := db.WithContext(dbCtx).Model(&models.DelegationTaskFeeLeaderboardSnapshot{}).
		Order("leaderboard_rank ASC").
		Limit(DelegationTaskFeeLeaderboardSize).
		Find(&snapshots).Error; err != nil {
		return nil, err
	}
	return snapshots, nil
}
