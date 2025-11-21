package blockchain

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

func GetNodeDelegatorShare(ctx context.Context, nodeAddress common.Address, network string) (uint8, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return 0, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	opts := &bind.CallOpts{
		Pending: false,
		Context: callCtx,
	}
	return client.DelegatedStakingContractInstance.GetNodeDelegatorShare(opts, nodeAddress)
}

func GetUserStakeAmountOfNode(ctx context.Context, nodeAddress common.Address, network string) (*big.Int, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	opts := &bind.CallOpts{
		Pending: false,
		Context: callCtx,
	}
	return client.DelegatedStakingContractInstance.GetNodeTotalStakeAmount(opts, nodeAddress)
}
