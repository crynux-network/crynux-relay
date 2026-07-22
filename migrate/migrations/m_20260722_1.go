package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegationTaskFeeLeaderboardSnapshotGPUNameForM20260722_1 struct {
	GPUName string `gorm:"type:string;size:191;not null;default:''"`
}

func (delegationTaskFeeLeaderboardSnapshotGPUNameForM20260722_1) TableName() string {
	return "delegation_task_fee_leaderboard_snapshots"
}

func M20260722_1(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260722_1",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AddColumn(&delegationTaskFeeLeaderboardSnapshotGPUNameForM20260722_1{}, "GPUName")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&delegationTaskFeeLeaderboardSnapshotGPUNameForM20260722_1{}, "GPUName")
			},
		},
	})
}
