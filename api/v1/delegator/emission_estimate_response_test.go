package delegator

import (
	"crynux_relay/models"
	"encoding/json"
	"math/big"
	"testing"
)

func TestDelegatorResponseIncludesEmissionEstimateFields(t *testing.T) {
	payload, err := json.Marshal(DelegatorInfo{
		EstimatedUpcomingDelegationEmission: models.BigInt{Int: *big.NewInt(1)},
		EmissionWeekStart:                   1767830400,
		EmissionWeekEnd:                     1768435200,
		EstimateUpdatedAt:                   1768000000,
	})
	if err != nil {
		t.Fatalf("marshal delegator: %v", err)
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal delegator: %v", err)
	}
	for _, field := range []string{
		"estimated_upcoming_delegation_emission",
		"emission_week_start",
		"emission_week_end",
		"estimate_updated_at",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing field %s in %s", field, payload)
		}
	}
}

func TestDelegationResponseIncludesEmissionEstimateFields(t *testing.T) {
	payload, err := json.Marshal(DelegationInfo{
		EstimatedUpcomingEmission: models.BigInt{Int: *big.NewInt(1)},
		EmissionWeekStart:         1767830400,
		EmissionWeekEnd:           1768435200,
		EstimateUpdatedAt:         1768000000,
	})
	if err != nil {
		t.Fatalf("marshal delegation: %v", err)
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal delegation: %v", err)
	}
	for _, field := range []string{
		"estimated_upcoming_emission",
		"emission_week_start",
		"emission_week_end",
		"estimate_updated_at",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing field %s in %s", field, payload)
		}
	}
}
