package blockchain

import (
	"context"
	"crynux_relay/blockchain/bindings"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"gorm.io/gorm"
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

func GetNodeStakingInfos(ctx context.Context, nodeAddress common.Address, network string) ([]common.Address, []*big.Int, error) {
	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, nil, fmt.Errorf("network %s not found", network)
	}
	pageSize := blockchain.DelegatedStakingReadPageSize
	if pageSize == 0 {
		return nil, nil, fmt.Errorf("delegated staking read page size not configured for network %s", network)
	}

	count, err := GetNodeStakingInfoCount(ctx, nodeAddress, network)
	if err != nil {
		return nil, nil, err
	}
	if count.Sign() == 0 {
		return []common.Address{}, []*big.Int{}, nil
	}

	addresses := make([]common.Address, 0, count.Int64())
	amounts := make([]*big.Int, 0, count.Int64())
	for page := uint64(1); uint64(len(addresses)) < count.Uint64(); page++ {
		pageAddresses, pageAmounts, err := GetNodeStakingInfoPage(ctx, nodeAddress, network, page, pageSize)
		if err != nil {
			return nil, nil, err
		}
		if len(pageAddresses) == 0 {
			break
		}
		addresses = append(addresses, pageAddresses...)
		amounts = append(amounts, pageAmounts...)
	}
	return addresses, amounts, nil
}

func GetNodeStakingInfoCount(ctx context.Context, nodeAddress common.Address, network string) (*big.Int, error) {
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
	return client.DelegatedStakingContractInstance.GetNodeStakingInfoCount(opts, nodeAddress)
}

func GetNodeStakingInfoPage(ctx context.Context, nodeAddress common.Address, network string, page, pageSize uint64) ([]common.Address, []*big.Int, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return nil, nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	opts := &bind.CallOpts{
		Pending: false,
		Context: callCtx,
	}
	return client.DelegatedStakingContractInstance.GetNodeStakingInfos(opts, nodeAddress, new(big.Int).SetUint64(page), new(big.Int).SetUint64(pageSize))
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

func SlashNodeDelegations(ctx context.Context, db *gorm.DB, nodeAddress common.Address, delegators []common.Address, network string) (*models.BlockchainTransaction, error) {
	if len(delegators) == 0 {
		return nil, fmt.Errorf("delegator batch is empty")
	}

	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, fmt.Errorf("network %s not found", network)
	}

	abi, err := bindings.DelegatedStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	data, err := abi.Pack("slashNodeDelegations", nodeAddress, delegators)
	if err != nil {
		return nil, err
	}
	dataStr := hexutil.Encode(data)

	transaction := &models.BlockchainTransaction{
		Network:     network,
		Type:        "DelegatedStaking::slashNodeDelegations",
		Status:      models.TransactionStatusPending,
		FromAddress: blockchain.Account.Address,
		ToAddress:   blockchain.Contracts.DelegatedStaking,
		Value:       "0",
		Data: sql.NullString{
			String: dataStr,
			Valid:  true,
		},
		MaxRetries: blockchain.MaxRetries,
	}

	if err := transaction.Save(ctx, db); err != nil {
		return nil, err
	}

	return transaction, nil
}
