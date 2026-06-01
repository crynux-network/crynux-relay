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

// GetMinStakeAmount gets the minimum stake amount
func GetMinStakeAmount(ctx context.Context, network string) (*big.Int, error) {
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

	return client.NodeStakingContractInstance.GetMinStakeAmount(opts)
}

// GetStakingInfo gets the staking information for a specific node
func GetStakingInfo(ctx context.Context, nodeAddress common.Address, network string) (bindings.NodeStakingStakingInfo, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return bindings.NodeStakingStakingInfo{}, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	opts := &bind.CallOpts{
		Pending: false,
		Context: callCtx,
	}

	return client.NodeStakingContractInstance.GetStakingInfo(opts, nodeAddress)
}

// GetNodeStakingOwner gets the contract owner address
func GetNodeStakingOwner(ctx context.Context, network string) (common.Address, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return common.Address{}, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	opts := &bind.CallOpts{
		Pending: false,
		Context: callCtx,
	}

	return client.NodeStakingContractInstance.Owner(opts)
}

// Stake stakes tokens
func Stake(ctx context.Context, stakedAmount *big.Int, network string) (string, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return "", err
	}

	client.NonceMu.Lock()
	defer client.NonceMu.Unlock()

	nonce, err := client.GetNonce(ctx)
	if err != nil {
		return "", err
	}

	auth, err := client.GetAuth(ctx)
	if err != nil {
		return "", err
	}

	// Set the stake amount as the transaction value
	auth.Value = stakedAmount

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Limiter.Wait(callCtx); err != nil {
		return "", err
	}
	auth.Context = callCtx
	auth.Nonce = big.NewInt(int64(nonce))

	tx, err := client.NodeStakingContractInstance.Stake(auth, stakedAmount)
	if err != nil {
		err = client.processSendingTxError(err)
		return "", err
	}

	client.IncrementNonce()
	return tx.Hash().Hex(), nil
}

// QueueStake queues a stake transaction to be sent later
func QueueStake(ctx context.Context, db *gorm.DB, stakedAmount *big.Int, network string) (*models.BlockchainTransaction, error) {
	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, fmt.Errorf("network %s not found", network)
	}

	abi, err := bindings.NodeStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	data, err := abi.Pack("stake", stakedAmount)
	if err != nil {
		return nil, err
	}
	dataStr := hexutil.Encode(data)

	transaction := &models.BlockchainTransaction{
		Network:     network,
		Type:        "NodeStaking::stake",
		Status:      models.TransactionStatusPending,
		FromAddress: blockchain.Account.Address,
		ToAddress:   blockchain.Contracts.NodeStaking,
		Value:       stakedAmount.String(),
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

// Unstake removes the stake
func Unstake(ctx context.Context, nodeAddress common.Address, network string) (string, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return "", err
	}

	client.NonceMu.Lock()
	defer client.NonceMu.Unlock()

	nonce, err := client.GetNonce(ctx)
	if err != nil {
		return "", err
	}

	auth, err := client.GetAuth(ctx)
	if err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Limiter.Wait(callCtx); err != nil {
		return "", err
	}
	auth.Context = callCtx
	auth.Nonce = big.NewInt(int64(nonce))

	tx, err := client.NodeStakingContractInstance.Unstake(auth, nodeAddress)
	if err != nil {
		err = client.processSendingTxError(err)
		return "", err
	}

	client.IncrementNonce()
	return tx.Hash().Hex(), nil
}

// QueueUnstake queues an unstake transaction to be sent later
func QueueUnstake(ctx context.Context, db *gorm.DB, nodeAddress common.Address, network string) (*models.BlockchainTransaction, error) {
	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, fmt.Errorf("network %s not found", network)
	}

	abi, err := bindings.NodeStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	data, err := abi.Pack("unstake", nodeAddress)
	if err != nil {
		return nil, err
	}
	dataStr := hexutil.Encode(data)

	transaction := &models.BlockchainTransaction{
		Network:     network,
		Type:        "NodeStaking::unstake",
		Status:      models.TransactionStatusPending,
		FromAddress: blockchain.Account.Address,
		ToAddress:   blockchain.Contracts.NodeStaking,
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

// SetAdminAddress sets the admin address (owner only)
func SetAdminAddressForNodeStaking(ctx context.Context, adminAddress common.Address, network string) (string, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return "", err
	}

	client.NonceMu.Lock()
	defer client.NonceMu.Unlock()

	nonce, err := client.GetNonce(ctx)
	if err != nil {
		return "", err
	}

	auth, err := client.GetAuth(ctx)
	if err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Limiter.Wait(callCtx); err != nil {
		return "", err
	}
	auth.Context = callCtx
	auth.Nonce = big.NewInt(int64(nonce))

	tx, err := client.NodeStakingContractInstance.SetAdminAddress(auth, adminAddress)
	if err != nil {
		err = client.processSendingTxError(err)
		return "", err
	}

	client.IncrementNonce()
	return tx.Hash().Hex(), nil
}

// QueueSetAdminAddressForNodeStaking queues a set admin address transaction to be sent later
func QueueSetAdminAddressForNodeStaking(ctx context.Context, db *gorm.DB, adminAddress common.Address, network string) (*models.BlockchainTransaction, error) {
	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, fmt.Errorf("network %s not found", network)
	}

	abi, err := bindings.NodeStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	data, err := abi.Pack("setAdminAddress", adminAddress)
	if err != nil {
		return nil, err
	}
	dataStr := hexutil.Encode(data)

	transaction := &models.BlockchainTransaction{
		Network:     network,
		Type:        "NodeStaking::setAdminAddress",
		Status:      models.TransactionStatusPending,
		FromAddress: blockchain.Account.Address,
		ToAddress:   blockchain.Contracts.NodeStaking,
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

// SlashStaking slashes the node's stake (owner or admin only)
func SlashStaking(ctx context.Context, nodeAddress common.Address, network string) (string, error) {
	client, err := GetBlockchainClient(network)
	if err != nil {
		return "", err
	}

	client.NonceMu.Lock()
	defer client.NonceMu.Unlock()
	nonce, err := client.GetNonce(ctx)
	if err != nil {
		return "", err
	}

	auth, err := client.GetAuth(ctx)
	if err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := client.Limiter.Wait(callCtx); err != nil {
		return "", err
	}
	auth.Context = callCtx
	auth.Nonce = big.NewInt(int64(nonce))

	tx, err := client.NodeStakingContractInstance.SlashStaking(auth, nodeAddress)
	if err != nil {
		err = client.processSendingTxError(err)
		return "", err
	}

	client.IncrementNonce()
	return tx.Hash().Hex(), nil
}

// QueueSlashStaking queues a slash staking transaction to be sent later
func QueueSlashStaking(ctx context.Context, db *gorm.DB, nodeAddress common.Address, network string) (*models.BlockchainTransaction, error) {
	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, fmt.Errorf("network %s not found", network)
	}

	abi, err := bindings.NodeStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}

	data, err := abi.Pack("slashStaking", nodeAddress)
	if err != nil {
		return nil, err
	}
	dataStr := hexutil.Encode(data)

	transaction := &models.BlockchainTransaction{
		Network:     network,
		Type:        "NodeStaking::slashStaking",
		Status:      models.TransactionStatusPending,
		FromAddress: blockchain.Account.Address,
		ToAddress:   blockchain.Contracts.NodeStaking,
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
