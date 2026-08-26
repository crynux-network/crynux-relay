package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type vestingRecordFixtureForM20260826 struct {
	ID             uint `gorm:"primaryKey"`
	DeletedAt      *time.Time
	Status         int8
	TotalAmount    string
	ReleasedAmount string
	DurationDays   uint
	Type           string
}

func (vestingRecordFixtureForM20260826) TableName() string {
	return "vesting_records"
}

type vestingDelegationDetailFixtureForM20260826 struct {
	ID              uint `gorm:"primaryKey"`
	VestingRecordID uint
	EmissionAmount  string
}

func (vestingDelegationDetailFixtureForM20260826) TableName() string {
	return "vesting_delegation_emission_details"
}

func TestM20260826DeprecatesActiveVestingsAndClearsDerivedTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(
		&vestingRecordFixtureForM20260826{},
		&vestingDelegationDetailFixtureForM20260826{},
		&nodeDelegationEmissionWeeklyTotalForM20260826{},
		&delegatedStakingNodeListSnapshotForM20260826{},
		&delegationTaskFeeLeaderboardSnapshotForM20260826{},
	); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	deletedAt := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	records := []vestingRecordFixtureForM20260826{
		{Status: 0, TotalAmount: "1000", ReleasedAmount: "100", DurationDays: 30, Type: "node"},
		{Status: 0, TotalAmount: "2000", ReleasedAmount: "200", DurationDays: 180, Type: "delegation"},
		{Status: 1, TotalAmount: "3000", ReleasedAmount: "3000", DurationDays: 1, Type: "other"},
		{DeletedAt: &deletedAt, Status: 0, TotalAmount: "4000", ReleasedAmount: "400", DurationDays: 365, Type: "node"},
	}
	if err := db.Create(&records).Error; err != nil {
		t.Fatalf("create vestings: %v", err)
	}
	detail := vestingDelegationDetailFixtureForM20260826{
		VestingRecordID: records[1].ID,
		EmissionAmount:  "2000",
	}
	if err := db.Create(&detail).Error; err != nil {
		t.Fatalf("create delegation detail: %v", err)
	}
	for _, row := range []interface{}{
		&nodeDelegationEmissionWeeklyTotalForM20260826{},
		&delegatedStakingNodeListSnapshotForM20260826{},
		&delegationTaskFeeLeaderboardSnapshotForM20260826{},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create derived row: %v", err)
		}
	}

	migration := M20260826(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var got []vestingRecordFixtureForM20260826
	if err := db.Order("id ASC").Find(&got).Error; err != nil {
		t.Fatalf("list vestings: %v", err)
	}
	expectedStatuses := []int8{2, 2, 1, 0}
	for i, expected := range expectedStatuses {
		if got[i].Status != expected {
			t.Fatalf("record %d status: expected %d, got %d", i, expected, got[i].Status)
		}
		if got[i].TotalAmount != records[i].TotalAmount || got[i].ReleasedAmount != records[i].ReleasedAmount {
			t.Fatalf("record %d amounts changed", i)
		}
	}

	var detailCount int64
	if err := db.Model(&vestingDelegationDetailFixtureForM20260826{}).Count(&detailCount).Error; err != nil {
		t.Fatalf("count delegation details: %v", err)
	}
	if detailCount != 1 {
		t.Fatalf("expected delegation detail to remain, got %d", detailCount)
	}
	for name, table := range map[string]interface{}{
		"weekly totals":         &nodeDelegationEmissionWeeklyTotalForM20260826{},
		"node list snapshots":   &delegatedStakingNodeListSnapshotForM20260826{},
		"leaderboard snapshots": &delegationTaskFeeLeaderboardSnapshotForM20260826{},
	} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("expected %s to be empty, got %d", name, count)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var deprecatedCount int64
	if err := db.Model(&vestingRecordFixtureForM20260826{}).Where("status = ?", 2).Count(&deprecatedCount).Error; err != nil {
		t.Fatalf("count deprecated after rollback: %v", err)
	}
	if deprecatedCount != 2 {
		t.Fatalf("rollback changed migrated records: got %d deprecated", deprecatedCount)
	}
}
