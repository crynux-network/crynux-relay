package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type delegationTaskFeeLeaderboardSnapshotTableForM20260722_1 struct {
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

func (delegationTaskFeeLeaderboardSnapshotTableForM20260722_1) TableName() string {
	return "delegation_task_fee_leaderboard_snapshots"
}

func TestM20260722_1AddsGPUName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&delegationTaskFeeLeaderboardSnapshotTableForM20260722_1{}); err != nil {
		t.Fatalf("failed to create delegation_task_fee_leaderboard_snapshots table: %v", err)
	}

	migration := M20260722_1(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&delegationTaskFeeLeaderboardSnapshotGPUNameForM20260722_1{}, "GPUName") {
		t.Fatalf("expected delegation_task_fee_leaderboard_snapshots.gpu_name column to exist")
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&delegationTaskFeeLeaderboardSnapshotGPUNameForM20260722_1{}, "GPUName") {
		t.Fatalf("expected delegation_task_fee_leaderboard_snapshots.gpu_name column to be dropped")
	}
}
