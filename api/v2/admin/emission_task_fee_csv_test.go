package admin

import (
	"math/big"
	"testing"
)

func TestBuildEmissionTaskFeeCSVRowsAllocatesIntegerCNXAndRemainder(t *testing.T) {
	wei := big.NewInt(1_000_000_000_000_000_000)
	participants := []emissionTaskFeeParticipant{
		{
			Address: "0xnode",
			Type:    "node",
			TaskFee: big.NewInt(0).Mul(big.NewInt(2), wei),
		},
		{
			Address:     "0xdelegator",
			Type:        "delegation",
			TaskFee:     big.NewInt(0).Mul(big.NewInt(1), wei),
			NodeAddress: "0xnode",
			Network:     "base",
		},
	}
	total := big.NewInt(0).Mul(big.NewInt(3), wei)

	rows := buildEmissionTaskFeeCSVRows(participants, total, 10, "1767830400")
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	if rows[0].Address != "0xnode" || rows[0].Type != "node" || rows[0].TaskFee != "2.000000" || rows[0].Emission != "6.000000" || rows[0].StartTime != "1767830400" {
		t.Fatalf("unexpected node row: %+v", rows[0])
	}
	if rows[1].Address != "0xdelegator" || rows[1].Type != "delegation" || rows[1].TaskFee != "1.000000" || rows[1].Emission != "3.000000" || rows[1].StartTime != "1767830400" {
		t.Fatalf("unexpected delegator row: %+v", rows[1])
	}
	if rows[1].NodeAddress != "0xnode" || rows[1].Network != "base" {
		t.Fatalf("unexpected delegator detail columns: %+v", rows[1])
	}
	if rows[2].Type != "remainder" || rows[2].TaskFee != "0.000000" || rows[2].Emission != "1.000000" || rows[2].StartTime != "1767830400" {
		t.Fatalf("unexpected remainder row: %+v", rows[2])
	}
}

func TestBuildEmissionTaskFeeCSVRowsHandlesZeroTotal(t *testing.T) {
	rows := buildEmissionTaskFeeCSVRows(nil, big.NewInt(0), 7, "1767830400")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Type != "remainder" || rows[0].TaskFee != "0.000000" || rows[0].Emission != "7.000000" || rows[0].StartTime != "1767830400" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

func TestBuildEmissionTaskFeeCSVRowsSkipsSubOneCNXEmission(t *testing.T) {
	participants := []emissionTaskFeeParticipant{
		{
			Address: "0xnode",
			Type:    "node",
			TaskFee: big.NewInt(1),
		},
	}

	rows := buildEmissionTaskFeeCSVRows(participants, big.NewInt(10), 1, "1767830400")
	if len(rows) != 1 {
		t.Fatalf("expected only remainder row, got %d", len(rows))
	}
	if rows[0].Type != "remainder" || rows[0].Emission != "1.000000" || rows[0].StartTime != "1767830400" {
		t.Fatalf("unexpected remainder row: %+v", rows[0])
	}
}
