package service

import (
	"crynux_relay/blockchain/bindings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestProcessNodeStakingReceiptLogsHandlesNodeSlashed(t *testing.T) {
	nodeAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	receiptLog := &types.Log{}

	var slashedNode common.Address
	err := processNodeStakingReceiptLogsWithParsers([]*types.Log{receiptLog}, nodeStakingReceiptLogParsers{
		parseNodeSlashed: func(receiptLog types.Log) (*bindings.NodeStakingNodeSlashed, error) {
			return &bindings.NodeStakingNodeSlashed{NodeAddress: nodeAddress}, nil
		},
	}, nodeStakingReceiptLogHandlers{
		onNodeSlashed: func(event *bindings.NodeStakingNodeSlashed) error {
			slashedNode = event.NodeAddress
			return nil
		},
	})
	if err != nil {
		t.Fatalf("processNodeStakingReceiptLogs returned error: %v", err)
	}
	if slashedNode != nodeAddress {
		t.Fatalf("expected node slash handler to receive node %s, got %s", nodeAddress.Hex(), slashedNode.Hex())
	}
}
