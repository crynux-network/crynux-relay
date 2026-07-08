package models

import (
	"context"
	"math/big"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpsertNodeDelegationEmissionWeeklyTotalIncrementsAddsExistingTotals(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&NodeDelegationEmissionWeeklyTotal{}); err != nil {
		t.Fatalf("failed to migrate node delegation emission weekly total: %v", err)
	}

	ctx := context.Background()
	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	first := []NodeDelegationEmissionWeeklyIncrement{
		{
			NodeAddress:    "0xnode",
			StartTime:      start,
			EmissionAmount: big.NewInt(100),
		},
	}
	if err := UpsertNodeDelegationEmissionWeeklyTotalIncrements(ctx, db, first); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	second := []NodeDelegationEmissionWeeklyIncrement{
		{
			NodeAddress:    "0xnode",
			StartTime:      start,
			EmissionAmount: big.NewInt(250),
		},
		{
			NodeAddress:    "0xnode",
			StartTime:      start.Add(7 * 24 * time.Hour),
			EmissionAmount: big.NewInt(300),
		},
	}
	if err := UpsertNodeDelegationEmissionWeeklyTotalIncrements(ctx, db, second); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	totals, err := ListNodeDelegationEmissionWeeklyTotalsByNodeAndStartTimeRange(ctx, db, "0xnode", start, start.Add(14*24*time.Hour))
	if err != nil {
		t.Fatalf("list totals failed: %v", err)
	}
	if len(totals) != 2 {
		t.Fatalf("expected 2 totals, got %d", len(totals))
	}
	if totals[0].EmissionAmount.Int.Cmp(big.NewInt(350)) != 0 {
		t.Fatalf("expected first total 350, got %s", totals[0].EmissionAmount.String())
	}
	if totals[1].EmissionAmount.Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected second total 300, got %s", totals[1].EmissionAmount.String())
	}
}
