package incentive

import (
	"encoding/json"
	"testing"
)

func TestDelegationIncentiveResponseFields(t *testing.T) {
	payload, err := json.Marshal(DelegationIncentive{
		DelegatorAddress: "0xdelegator",
		NodeAddress:      "0xnode",
		Network:          "base",
		StakingAmount:    "1000",
		TaskFee:          "300",
		DelegationApr12m: 0.42,
	})
	if err != nil {
		t.Fatalf("marshal delegation incentive: %v", err)
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal delegation incentive: %v", err)
	}
	for _, field := range []string{
		"delegator_address",
		"node_address",
		"network",
		"staking_amount",
		"task_fee",
		"delegation_apr_12m",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing field %s in %s", field, payload)
		}
	}
}
