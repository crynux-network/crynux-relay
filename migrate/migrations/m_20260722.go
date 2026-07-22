package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegationTaskFeeLeaderboardSnapshotForM20260722 struct {
	ID               uint `gorm:"primarykey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Rank             uint8   `gorm:"column:leaderboard_rank;not null;index"`
	DelegatorAddress string  `gorm:"type:string;size:191;not null"`
	NodeAddress      string  `gorm:"type:string;size:191;not null"`
	Network          string  `gorm:"type:string;size:64;not null"`
	StakingAmount    string  `gorm:"type:decimal(65,0);not null;default:0"`
	TaskFee          string  `gorm:"type:decimal(65,0);not null;default:0"`
	DelegationApr12m float64 `gorm:"column:delegation_apr_12m;not null;default:0"`
}

func (delegationTaskFeeLeaderboardSnapshotForM20260722) TableName() string {
	return "delegation_task_fee_leaderboard_snapshots"
}

func M20260722(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260722",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().CreateTable(&delegationTaskFeeLeaderboardSnapshotForM20260722{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&delegationTaskFeeLeaderboardSnapshotForM20260722{})
			},
		},
	})
}
