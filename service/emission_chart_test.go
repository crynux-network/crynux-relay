package service

import (
	"crynux_relay/models"
	"math/big"
	"testing"
	"time"
)

func TestBuildEmissionIncomeSeriesAggregatesNormalizedWeekStarts(t *testing.T) {
	chartRange := &EmissionChartRange{
		MainnetWeekStart: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
		RangeStart:       time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		RangeEnd:         time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC),
		WeekStarts: []time.Time{
			time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC),
		},
	}

	records := []models.VestingRecord{
		{
			StartTime:   time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(100)},
		},
		{
			StartTime:   time.Date(2026, 1, 18, 5, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(200)},
		},
		{
			StartTime:   time.Date(2026, 1, 20, 8, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(300)},
		},
	}

	timestamps, emissionIncome := BuildEmissionIncomeSeries(records, chartRange)
	if len(timestamps) != 2 || len(emissionIncome) != 2 {
		t.Fatalf("expected 2 points, got timestamps=%d amounts=%d", len(timestamps), len(emissionIncome))
	}

	if emissionIncome[0].Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected first week amount 300, got %s", emissionIncome[0].String())
	}
	if emissionIncome[1].Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected second week amount 300, got %s", emissionIncome[1].String())
	}
}

func TestBuildDelegationEmissionIncomeSeriesAggregatesAndZeroFillsWeeks(t *testing.T) {
	chartRange := &EmissionChartRange{
		MainnetWeekStart: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
		RangeStart:       time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		RangeEnd:         time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		WeekStarts: []time.Time{
			time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC),
		},
	}

	details := []models.VestingDelegationEmissionDetail{
		{
			StartTime:      time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC),
			EmissionAmount: models.BigInt{Int: *big.NewInt(100)},
		},
		{
			StartTime:      time.Date(2026, 1, 18, 5, 0, 0, 0, time.UTC),
			EmissionAmount: models.BigInt{Int: *big.NewInt(200)},
		},
		{
			StartTime:      time.Date(2026, 1, 28, 8, 0, 0, 0, time.UTC),
			EmissionAmount: models.BigInt{Int: *big.NewInt(300)},
		},
	}

	timestamps, emissionIncome := BuildDelegationEmissionIncomeSeries(details, chartRange)
	if len(timestamps) != 3 || len(emissionIncome) != 3 {
		t.Fatalf("expected 3 points, got timestamps=%d amounts=%d", len(timestamps), len(emissionIncome))
	}

	if emissionIncome[0].Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected first week amount 300, got %s", emissionIncome[0].String())
	}
	if emissionIncome[1].Int.Sign() != 0 {
		t.Fatalf("expected second week amount 0, got %s", emissionIncome[1].String())
	}
	if emissionIncome[2].Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected third week amount 300, got %s", emissionIncome[2].String())
	}
}

func TestBuildNodeDelegationEmissionIncomeSeriesUsesAggregateTotalsAndZeroFillsWeeks(t *testing.T) {
	chartRange := &EmissionChartRange{
		MainnetWeekStart: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
		RangeStart:       time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		RangeEnd:         time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		WeekStarts: []time.Time{
			time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC),
		},
	}

	totals := []models.NodeDelegationEmissionWeeklyTotal{
		{
			StartTime:      time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			EmissionAmount: models.BigInt{Int: *big.NewInt(300)},
		},
		{
			StartTime:      time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC),
			EmissionAmount: models.BigInt{Int: *big.NewInt(700)},
		},
	}

	timestamps, emissionIncome := BuildNodeDelegationEmissionIncomeSeries(totals, chartRange)
	if len(timestamps) != 3 || len(emissionIncome) != 3 {
		t.Fatalf("expected 3 points, got timestamps=%d amounts=%d", len(timestamps), len(emissionIncome))
	}
	if emissionIncome[0].Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected first week amount 300, got %s", emissionIncome[0].String())
	}
	if emissionIncome[1].Int.Sign() != 0 {
		t.Fatalf("expected second week amount 0, got %s", emissionIncome[1].String())
	}
	if emissionIncome[2].Int.Cmp(big.NewInt(700)) != 0 {
		t.Fatalf("expected third week amount 700, got %s", emissionIncome[2].String())
	}
}

func TestBuildTypedEmissionIncomeSeriesExcludesOtherType(t *testing.T) {
	chartRange := &EmissionChartRange{
		MainnetWeekStart: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
		RangeStart:       time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		RangeEnd:         time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC),
		WeekStarts: []time.Time{
			time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC),
		},
	}

	records := []models.VestingRecord{
		{
			StartTime:   time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(100)},
			Type:        models.VestingTypeNode,
		},
		{
			StartTime:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(50)},
			Type:        models.VestingTypeDelegation,
		},
		{
			StartTime:   time.Date(2026, 1, 20, 8, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(70)},
			Type:        models.VestingTypeDelegation,
		},
		{
			StartTime:   time.Date(2026, 1, 20, 9, 0, 0, 0, time.UTC),
			TotalAmount: models.BigInt{Int: *big.NewInt(999)},
			Type:        models.VestingTypeOther,
		},
	}

	series := BuildTypedEmissionIncomeSeries(records, chartRange)
	if len(series.Timestamps) != 2 {
		t.Fatalf("expected 2 timestamps, got %d", len(series.Timestamps))
	}
	if len(series.NodeEmissionIncome) != 2 || len(series.DelegationEmissionIncome) != 2 {
		t.Fatalf("expected 2 typed points, got node=%d delegation=%d", len(series.NodeEmissionIncome), len(series.DelegationEmissionIncome))
	}
	if series.NodeEmissionIncome[0].Int.Cmp(big.NewInt(100)) != 0 || series.NodeEmissionIncome[1].Int.Sign() != 0 {
		t.Fatalf("unexpected node series: %s %s", series.NodeEmissionIncome[0].String(), series.NodeEmissionIncome[1].String())
	}
	if series.DelegationEmissionIncome[0].Int.Cmp(big.NewInt(50)) != 0 || series.DelegationEmissionIncome[1].Int.Cmp(big.NewInt(70)) != 0 {
		t.Fatalf("unexpected delegation series: %s %s", series.DelegationEmissionIncome[0].String(), series.DelegationEmissionIncome[1].String())
	}
}
