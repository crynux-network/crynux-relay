package nodes

import (
	"crynux_relay/models"
	"encoding/json"
	"math/big"
	"testing"
)

func TestHasNodeEmissionAccessReturnsFalseWhenDelegatorShareDisabled(t *testing.T) {
	node := &models.Node{
		Address:        "0xnode",
		Network:        "ethereum-sepolia",
		DelegatorShare: 0,
	}

	if hasNodeEmissionAccess(node, "ethereum-sepolia") {
		t.Fatal("expected access denied when delegator share is disabled")
	}
}

func TestNodeEmissionChartDataUsesNodeEmissionIncomeField(t *testing.T) {
	payload, err := json.Marshal(NodeEmissionChartData{
		Timestamps: []int64{1},
		NodeEmissionIncome: []models.BigInt{
			{Int: *big.NewInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := data["node_emission_income"]; !ok {
		t.Fatalf("expected node_emission_income field, got %v", data)
	}
	if _, ok := data["emission_income"]; ok {
		t.Fatalf("did not expect legacy emission_income field, got %v", data)
	}
}
