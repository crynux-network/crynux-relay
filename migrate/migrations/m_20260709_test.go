package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotEstimatedNextAPRTestBase struct {
	ID                     uint `gorm:"primarykey"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
	NodeAddress            string `gorm:"type:string;size:191;not null;uniqueIndex:idx_delegated_staking_node_snapshots_address"`
	DelegationApr12m       float64
	AprObservationDays     uint32
	DelegationAprUpdatedAt time.Time
}

func (delegatedStakingNodeListSnapshotEstimatedNextAPRTestBase) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func TestM20260709AddsEstimatedNextAPRColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&delegatedStakingNodeListSnapshotEstimatedNextAPRTestBase{}); err != nil {
		t.Fatalf("failed to migrate snapshot table: %v", err)
	}

	migration := M20260709(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	m := db.Migrator()
	for _, column := range []string{
		"EstimatedNext10kDelegationApr",
		"EstimatedNext100kDelegationApr",
		"EstimatedNext1mDelegationApr",
	} {
		if !m.HasColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, column) {
			t.Fatalf("expected column %s", column)
		}
		if !m.HasIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, column) {
			t.Fatalf("expected index %s", column)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	for _, column := range []string{
		"EstimatedNext10kDelegationApr",
		"EstimatedNext100kDelegationApr",
		"EstimatedNext1mDelegationApr",
	} {
		if m.HasColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, column) {
			t.Fatalf("expected column %s to be dropped", column)
		}
	}
}
