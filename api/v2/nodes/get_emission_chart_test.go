package nodes

import (
	"crynux_relay/models"
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
