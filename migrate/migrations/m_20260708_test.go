package migrations

import (
	"crynux_relay/models"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260708BackfillsNodeDelegationEmissionWeeklyTotals(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.VestingDelegationEmissionDetail{}); err != nil {
		t.Fatalf("failed to migrate detail table: %v", err)
	}

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	details := []models.VestingDelegationEmissionDetail{
		{
			VestingRecordID: 1,
			UserAddress:     "0xuser1",
			NodeAddress:     "0xnode",
			Network:         "base",
			TaskFee:         models.BigInt{Int: *big.NewInt(10)},
			EmissionAmount:  models.BigInt{Int: *big.NewInt(100)},
			StartTime:       start,
		},
		{
			VestingRecordID: 2,
			UserAddress:     "0xuser2",
			NodeAddress:     "0xnode",
			Network:         "near",
			TaskFee:         models.BigInt{Int: *big.NewInt(20)},
			EmissionAmount:  models.BigInt{Int: *big.NewInt(200)},
			StartTime:       start,
		},
		{
			VestingRecordID: 3,
			UserAddress:     "0xuser3",
			NodeAddress:     "0xother",
			Network:         "base",
			TaskFee:         models.BigInt{Int: *big.NewInt(30)},
			EmissionAmount:  models.BigInt{Int: *big.NewInt(300)},
			StartTime:       start,
		},
	}
	if err := db.Create(&details).Error; err != nil {
		t.Fatalf("failed to seed details: %v", err)
	}

	if err := M20260708(db).Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var totals []nodeDelegationEmissionWeeklyTotalMigration
	if err := db.Order("node_address").Find(&totals).Error; err != nil {
		t.Fatalf("failed to list totals: %v", err)
	}
	if len(totals) != 2 {
		t.Fatalf("expected 2 aggregate rows, got %d", len(totals))
	}
	if totals[0].NodeAddress != "0xnode" || totals[0].EmissionAmount != "300" {
		t.Fatalf("unexpected first total: %+v", totals[0])
	}
	if totals[1].NodeAddress != "0xother" || totals[1].EmissionAmount != "300" {
		t.Fatalf("unexpected second total: %+v", totals[1])
	}
}
