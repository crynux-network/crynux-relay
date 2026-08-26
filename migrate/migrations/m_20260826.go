package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const (
	vestingStatusActiveForM20260826     = int8(0)
	vestingStatusDeprecatedForM20260826 = int8(2)
)

type vestingRecordForM20260826 struct {
	ID        uint
	DeletedAt *time.Time
	Status    int8
}

func (vestingRecordForM20260826) TableName() string {
	return "vesting_records"
}

type nodeDelegationEmissionWeeklyTotalForM20260826 struct {
	ID uint
}

func (nodeDelegationEmissionWeeklyTotalForM20260826) TableName() string {
	return "node_delegation_emission_weekly_totals"
}

type delegatedStakingNodeListSnapshotForM20260826 struct {
	ID uint
}

func (delegatedStakingNodeListSnapshotForM20260826) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

type delegationTaskFeeLeaderboardSnapshotForM20260826 struct {
	ID uint
}

func (delegationTaskFeeLeaderboardSnapshotForM20260826) TableName() string {
	return "delegation_task_fee_leaderboard_snapshots"
}

func M20260826(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20260826",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Model(&vestingRecordForM20260826{}).
				Where("deleted_at IS NULL").
				Where("status = ?", vestingStatusActiveForM20260826).
				Update("status", vestingStatusDeprecatedForM20260826).Error; err != nil {
				return err
			}
			for _, table := range []interface{}{
				&nodeDelegationEmissionWeeklyTotalForM20260826{},
				&delegatedStakingNodeListSnapshotForM20260826{},
				&delegationTaskFeeLeaderboardSnapshotForM20260826{},
			} {
				if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(table).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(*gorm.DB) error {
			return nil
		},
	}})
}
