package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/blockchain/bindings"
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"

	"gorm.io/gorm"
)

// StartBlockListener starts the native token transfer listener
func StartBlockListener(ctx context.Context) {
	appConfig := config.GetConfig()

	// Start the listener goroutine
	for network := range appConfig.Blockchains {
		go func(network string) {
			if err := runBlockListener(ctx, config.GetDB(), network); err != nil {
				log.Errorf("Native token listener failed: %v", err)
			}
		}(network)
	}

	log.Info("Native token listener started")
}

// runBlockListener runs the native token transfer listener
func runBlockListener(ctx context.Context, db *gorm.DB, network string) error {
	ticker := time.NewTicker(5 * time.Second) // Check for new blocks every 5 seconds
	defer ticker.Stop()

	client, err := blockchain.GetBlockchainClient(network)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := processNewBlocks(ctx, db, client); err != nil {
				log.Errorf("Failed to process new blocks: %v", err)
			}
		}
	}
}

// processNewBlocks processes new blocks
func processNewBlocks(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient) error {
	// Get current block height
	latestBlock, err := client.RpcClient.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	// Get listener status
	listener, err := models.GetBlockListener(ctx, db, client.Network)
	if err != nil {
		return err
	}

	// If already at the latest block, skip
	if listener.LastBlockNum >= latestBlock.NumberU64() {
		return nil
	}

	// Process new blocks
	processedBlock := listener.LastBlockNum
	startBlock := listener.LastBlockNum + 1
	endBlock := latestBlock.NumberU64()

	// Limit the number of blocks processed each time to avoid long processing time
	if endBlock-startBlock > 10 {
		endBlock = startBlock + 10
	}

	log.Infof("Processing blocks from %d to %d", startBlock, endBlock)

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		if err := processBlock(ctx, db, client, blockNum); err != nil {
			log.Errorf("Failed to process block %d: %v", blockNum, err)
			break
		}
		processedBlock = blockNum
	}

	// Update listener status
	if err := db.Model(&listener).Updates(map[string]interface{}{
		"last_block_num":   processedBlock,
		"last_update_time": time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("failed to update block listener: %w", err)
	}

	return nil
}

// processBlock processes a single block
func processBlock(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient, blockNum uint64) error {
	block, err := client.RpcClient.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
	if err != nil {
		return fmt.Errorf("failed to get block %d: %w", blockNum, err)
	}

	// Check transactions in the block
	for _, tx := range block.Transactions() {
		if err := processTransaction(ctx, db, tx, client); err != nil {
			log.Errorf("Failed to process transaction %s: %v", tx.Hash().Hex(), err)
			return err
		}
	}

	return nil
}

func processBuyQuotaTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient) error {

	// Check if transaction is successful
	receipt, err := client.RpcClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	// Check if already processed
	event, err := models.GetTaskQuotaBoughtEvent(ctx, db, tx.Hash().Hex(), client.Network)
	if err != nil {
		return err
	}
	if event != nil {
		return nil
	}

	// Get sender address (need to recover from signature)
	from, err := types.Sender(types.LatestSignerForChainID(client.ChainID), tx)
	if err != nil {
		return fmt.Errorf("failed to get sender address of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	// Call BuyTaskQuota to add quota for the sender
	commitFunc, err := buyTaskQuota(ctx, db, tx.Hash().Hex(), from.Hex(), tx.Value(), client.Network)
	if err != nil {
		log.Errorf("Failed to buy task quota for %s, network: %s, error: %v", from.Hex(), client.Network, err)
		return err
	}

	// Execute quota update
	if err := commitFunc(); err != nil {
		log.Errorf("Failed to buy task quota for %s, network: %s, error: %v", from.Hex(), client.Network, err)
		return err
	}
	return nil
}

func processBuyTaskFeeTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient) error {
	// Check if transaction is successful
	receipt, err := client.RpcClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	// Check if already processed
	event, err := models.GetTaskFeeBoughtEvent(ctx, db, tx.Hash().Hex(), client.Network)
	if err != nil {
		return err
	}
	if event != nil {
		return nil
	}

	// Get sender address (need to recover from signature)
	from, err := types.Sender(types.LatestSignerForChainID(client.ChainID), tx)
	if err != nil {
		return fmt.Errorf("failed to get sender address of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	// Call BuyTaskQuota to add quota for the sender
	commitFunc, err := buyTaskFee(ctx, db, tx.Hash().Hex(), from.Hex(), tx.Value(), client.Network)
	if err != nil {
		log.Errorf("Failed to buy task fee for %s, network: %s, error: %v", from.Hex(), client.Network, err)
		return err
	}

	// Execute quota update
	if err := commitFunc(); err != nil {
		log.Errorf("Failed to buy task fee for %s, network: %s, error: %v", from.Hex(), client.Network, err)
		return err
	}
	return nil
}

func processNodeStakingTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient) error {
	receipt, err := client.RpcClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	for _, log := range receipt.Logs {
		event, err := client.NodeStakingContractInstance.ParseNodeStaked(*log)
		if err != nil {
			continue
		}
		if err := updateNodeStaking(ctx, db, event, client.Network); err != nil {
			return err
		}
	}

	return nil
}

func updateNodeStaking(ctx context.Context, db *gorm.DB, event *bindings.NodeStakingNodeStaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	address := event.NodeAddress.Hex()
	stakingAmount := big.NewInt(0).Add(event.StakedBalance, event.StakedCredits)
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var node models.Node
		if err := tx.Model(&models.Node{}).Where("address = ?", address).First(&node).Error; err != nil {
			return err
		}
		if node.Status == models.NodeStatusQuit {
			return nil
		}
		if err := tx.Model(&node).Update("stake_amount", models.BigInt{Int: *stakingAmount}).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.NodeStakingEvent{NodeAddress: address, StakingAmount: models.BigInt{Int: *stakingAmount}}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Errorf("UpdateNodeStaking: failed to update node staking for node %s: %v", address, err)
		return err
	}

	// Update value in memory
	totalStakeAmount := new(big.Int).Add(stakingAmount, GetUserStakeAmountOfNode(address, network))
	if totalStakeAmount.Sign() > 0 {
		UpdateMaxStaking(address, totalStakeAmount)
	}

	log.Infof("UpdateNodeStaking: successfully updated node %s stake amount to %s",
		address, stakingAmount.String())

	return nil
}

func processUserStakingTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient) error {
	receipt, err := client.RpcClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	for _, log := range receipt.Logs {
		if event, err := client.UserStakingContractInstance.ParseUserStaked(*log); err == nil {
			if err := updateUserStaking(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
		if event, err := client.UserStakingContractInstance.ParseUserUnstaked(*log); err == nil {
			if err := unstakeUserStaking(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
		if event, err := client.UserStakingContractInstance.ParseNodeDelegatorShareChanged(*log); err == nil {
			if err := changeNodeDelegatorShare(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
		if event, err := client.UserStakingContractInstance.ParseNodeSlashed(*log); err == nil {
			if err := slashUserStakingOfNode(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
	}

	return nil
}

func updateUserStaking(ctx context.Context, db *gorm.DB, event *bindings.UserStakingUserStaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	userAddress := event.UserAddress.Hex()
	nodeAddress := event.NodeAddress.Hex()
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userStaking models.UserStaking
		oldStakeAmount := big.NewInt(0)
		if err := tx.Model(&models.UserStaking{}).Where("user_address = ? AND node_address = ?", userAddress, nodeAddress).First(&userStaking).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				userStaking = models.UserStaking{
					UserAddress: userAddress,
					NodeAddress: nodeAddress,
					Amount:      models.BigInt{Int: *event.Amount},
					Valid:       true,
					Network:     network,
				}
			} else {
				return err
			}
		} else {
			if userStaking.Valid {
				oldStakeAmount.Set(&userStaking.Amount.Int)
			}
			userStaking.Amount = models.BigInt{Int: *event.Amount}
			userStaking.Valid = true
		}
		if err := tx.Save(&userStaking).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.UserStakingEvent{
			UserAddress: userAddress,
			NodeAddress: nodeAddress,
			Amount:      models.BigInt{Int: *event.Amount},
			Network:     network,
		}); err != nil {
			return err
		}
		UpdateUserStaking(userAddress, nodeAddress, event.Amount, network)
		return nil
	}); err != nil {
		log.Errorf("UpdateUserStaking: failed to update user staking %s -> %s: %v", userAddress, nodeAddress, err)
		return err
	}

	// Update value in memory
	if node, err := models.GetNodeByAddress(ctx, db, nodeAddress); err == nil {
		if node.Status != models.NodeStatusQuit {
			totalStakeAmount := new(big.Int).Add(&node.StakeAmount.Int, GetUserStakeAmountOfNode(nodeAddress, network))
			if totalStakeAmount.Sign() > 0 {
				UpdateMaxStaking(nodeAddress, totalStakeAmount)
			}

		}
	}

	log.Infof("UpdateUserStaking: successfully updated user %s stake amount to node %s: %s",
		userAddress, nodeAddress, event.Amount.String())

	return nil
}

func unstakeUserStaking(ctx context.Context, db *gorm.DB, event *bindings.UserStakingUserUnstaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	userAddress := event.UserAddress.Hex()
	nodeAddress := event.NodeAddress.Hex()

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userStaking models.UserStaking
		if err := tx.Model(&models.UserStaking{}).Where("user_address = ? AND node_address = ? AND network = ?", userAddress, nodeAddress, network).First(&userStaking).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		if !userStaking.Valid {
			return nil
		}
		if err := tx.Model(&userStaking).Update("valid", false).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.UserUnstakingEvent{
			UserAddress: userAddress,
			NodeAddress: nodeAddress,
			Amount:      userStaking.Amount,
			Network:     network,
		}); err != nil {
			return err
		}
		UnstakeUserStaking(userAddress, nodeAddress, network)
		return nil
	}); err != nil {
		log.Errorf("UnstakeUserStaking: failed to unstake user staking %s -> %s: %v", userAddress, nodeAddress, err)
		return err
	}

	if node, err := models.GetNodeByAddress(ctx, db, nodeAddress); err == nil {
		if node.Status != models.NodeStatusQuit {
			totalStakeAmount := new(big.Int).Add(&node.StakeAmount.Int, GetUserStakeAmountOfNode(nodeAddress, network))
			if totalStakeAmount.Sign() > 0 {
				UpdateMaxStaking(nodeAddress, totalStakeAmount)
			}

		}
	}

	log.Infof("UnstakeUserStaking: successfully unstake user staking %s -> %s",
		userAddress, nodeAddress)

	return nil
}

func changeNodeDelegatorShare(ctx context.Context, db *gorm.DB, event *bindings.UserStakingNodeDelegatorShareChanged, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	nodeAddress := event.NodeAddress.Hex()
	share := event.Share

	node, err := models.GetNodeByAddress(ctx, db, nodeAddress)
	if err != nil {
		log.Errorf("ChangeNodeDelegatorShare: failed to get node %s: %v", nodeAddress, err)
		return err
	}
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Node{}).Where("address = ?", nodeAddress).Where("network = ?", network).Update("delegator_share", share).Error; err != nil {
			return err
		}
		if share == 0 {
			// delete all user stakings to this node
			if err := tx.Model(&models.UserStaking{}).Where("node_address = ?", nodeAddress).Where("network = ?", network).Update("valid", false).Error; err != nil {
				return err
			}
			RemoveNodeUserStaking(nodeAddress, network)
		}
		if err := emitEvent(ctx, tx, &models.NodeDelegatorShareChangedEvent{
			NodeAddress: nodeAddress,
			Share:       share,
			Network:     network,
		}); err != nil {
			return err
		}
		if node.Network == network {
			SetDelegatorShare(nodeAddress, share)
		}
		return nil
	}); err != nil {
		log.Errorf("ChangeNodeDelegatorShare: failed to change delegator share of node %s: %v", nodeAddress, err)
		return err
	}

	if share == 0 && node.Status != models.NodeStatusQuit && node.Network == network{
		UpdateMaxStaking(nodeAddress, &node.StakeAmount.Int)
	}
	log.Infof("ChangeNodeDelegatorShare: successfully change delegator share of node %s to %d",
		nodeAddress, share)
	return nil
}

func slashUserStakingOfNode(ctx context.Context, db *gorm.DB, event *bindings.UserStakingNodeSlashed, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	nodeAddress := event.NodeAddress.Hex()
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.UserStaking{}).Where("node_address = ?", nodeAddress).Where("network = ?", network).Update("valid", false).Error; err != nil {
			return err
		}
		RemoveNodeUserStaking(nodeAddress, network)
		return nil
	}); err != nil {
		log.Errorf("SlashUserStakingOfNode: failed to slash user staking of node %s: %v", nodeAddress, err)
		return err
	}

	log.Infof("SlashUserStakingOfNode: successfully slash user staking of node: %s", nodeAddress)
	return nil
}

// processTransaction processes a single transaction
func processTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient) error {
	// Only process native token transfers (to field is not empty and data field is empty)
	if tx.To() == nil || len(tx.Data()) == 0 {
		return nil
	}

	appConfig := config.GetConfig()
	blockchainCfg, ok := appConfig.Blockchains[client.Network]
	if !ok {
		return fmt.Errorf("network %s not found", client.Network)
	}

	// Check if transfer is to the target address
	toAddress := tx.To().Hex()
	if strings.EqualFold(toAddress, appConfig.BuyQuota.Address) {
		return processBuyQuotaTransaction(ctx, db, tx, client)
	} else if strings.EqualFold(toAddress, appConfig.BuyTaskFee.Address) {
		return processBuyTaskFeeTransaction(ctx, db, tx, client)
	} else if strings.EqualFold(toAddress, blockchainCfg.Contracts.NodeStaking) {
		return processNodeStakingTransaction(ctx, db, tx, client)
	} else if strings.EqualFold(toAddress, blockchainCfg.Contracts.UserStaking) {
		return processUserStakingTransaction(ctx, db, tx, client)
	}

	return nil
}
