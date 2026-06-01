package service

import (
	"math/big"
	"testing"
)

func TestRemoveNodeDelegationsClearsUserSideCache(t *testing.T) {
	cache := newTestDelegationCache()
	delegatorA := "0x00000000000000000000000000000000000000AA"
	delegatorB := "0x00000000000000000000000000000000000000BB"
	nodeA := "0x00000000000000000000000000000000000000CC"
	nodeB := "0x00000000000000000000000000000000000000DD"

	cache.update(delegatorA, nodeA, big.NewInt(10))
	cache.update(delegatorB, nodeA, big.NewInt(20))
	cache.update(delegatorA, nodeB, big.NewInt(30))

	cache.removeNode(nodeA)

	if got := cache.getNodeTotalStakeAmount(nodeA); got.Sign() != 0 {
		t.Fatalf("expected removed node total staking to be 0, got %s", got.String())
	}
	if got := cache.getDelegatorTotalStakeAmount(delegatorA); got.Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("expected delegator A total staking to keep node B amount 30, got %s", got.String())
	}
	if got := cache.getDelegatorTotalStakeAmount(delegatorB); got.Sign() != 0 {
		t.Fatalf("expected delegator B total staking to be 0, got %s", got.String())
	}
	if got := cache.getDelegationsOfDelegator(delegatorA); len(got) != 1 || got[nodeB].Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("expected delegator A to keep only node B delegation, got %#v", got)
	}
	if got := cache.getDelegationsOfDelegator(delegatorB); len(got) != 0 {
		t.Fatalf("expected delegator B user-side delegations to be empty, got %#v", got)
	}
	if got := cache.getDelegationCountOfDelegator(delegatorA); got != 1 {
		t.Fatalf("expected delegator A delegation count to be 1, got %d", got)
	}
	if got := cache.getDelegationCountOfDelegator(delegatorB); got != 0 {
		t.Fatalf("expected delegator B delegation count to be 0, got %d", got)
	}
	if got := cache.getDelegatorCountOfNode(nodeA); got != 0 {
		t.Fatalf("expected removed node delegator count to be 0, got %d", got)
	}
}
