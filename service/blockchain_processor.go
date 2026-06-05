package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/blockchain/bindings"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	log "github.com/sirupsen/logrus"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StartBlockchainProcessors starts one chain processor per configured blockchain network.
func StartBlockchainProcessors(ctx context.Context) {
	appConfig := config.GetConfig()

	for network := range appConfig.Blockchains {
		go func(network string) {
			if err := runNodeBlockchainProcessor(ctx, config.GetDB(), network); err != nil {
				log.Errorf("Node blockchain processor failed: %v", err)
			}
		}(network)
	}
	for network := range appConfig.DepositWithdrawNetworks {
		go func(network string) {
			if err := runDepositWithdrawBlockchainProcessor(ctx, config.GetDB(), network); err != nil {
				log.Errorf("Deposit withdraw blockchain processor failed: %v", err)
			}
		}(network)
	}

	log.Info("Blockchain processors started")
}

// runNodeBlockchainProcessor runs block scanning for a node blockchain network.
func runNodeBlockchainProcessor(ctx context.Context, db *gorm.DB, network string) error {
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
			if err := processNodeNetworkBlockRange(ctx, db, client); err != nil {
				log.Errorf("Failed to process node network block range: %v", err)
			}
		}
	}
}

// runDepositWithdrawBlockchainProcessor runs ERC20 log scanning for a deposit and withdraw only network.
func runDepositWithdrawBlockchainProcessor(ctx context.Context, db *gorm.DB, network string) error {
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
			if err := processDepositWithdrawNetworkBlockRange(ctx, db, client); err != nil {
				log.Errorf("Failed to process deposit withdraw network block range: %v", err)
			}
		}
	}
}

func latestBlockNumber(ctx context.Context, client *blockchain.BlockchainClient) (uint64, error) {
	// Get current block height
	latestBlockNum, err := client.RpcClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}
	return latestBlockNum, nil
}

// processNodeNetworkBlockRange processes the next unprocessed block range for a node network.
func processNodeNetworkBlockRange(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient) error {
	latestBlockNum, err := latestBlockNumber(ctx, client)
	if err != nil {
		return err
	}

	appConfig := config.GetConfig()
	networkConfig, ok := appConfig.Blockchains[client.Network]
	if !ok {
		return fmt.Errorf("network %s not found", client.Network)
	}

	cursor, err := models.GetBlockchainCursor(ctx, db, client.Network, networkConfig.StartBlockNum)
	if err != nil {
		return err
	}

	// If already at the latest block, skip
	if cursor.LastBlockNum >= latestBlockNum {
		return nil
	}

	processedBlock := cursor.LastBlockNum
	startBlock := cursor.LastBlockNum + 1
	endBlock := latestBlockNum

	// Limit the number of blocks processed each time to avoid long processing time.
	if endBlock-startBlock > 10 {
		endBlock = startBlock + 10
	}

	log.Debugf("Processing blocks from %d to %d on %s", startBlock, endBlock, client.Network)

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		if err := processBlock(ctx, db, client, blockNum); err != nil {
			log.Errorf("Failed to process block %d: %v", blockNum, err)
			break
		}
		processedBlock = blockNum
	}

	// Update cursor status
	if err := db.Model(&cursor).Updates(map[string]interface{}{
		"last_block_num":   processedBlock,
		"last_update_time": time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("failed to update blockchain cursor: %w", err)
	}

	return nil
}

// processDepositWithdrawNetworkBlockRange processes the next ERC20 log range for a deposit and withdraw only network.
func processDepositWithdrawNetworkBlockRange(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient) error {
	latestBlockNum, err := latestBlockNumber(ctx, client)
	if err != nil {
		return err
	}

	appConfig := config.GetConfig()
	networkConfig, ok := appConfig.DepositWithdrawNetworks[client.Network]
	if !ok {
		return fmt.Errorf("deposit withdraw network %s not found", client.Network)
	}

	cursor, err := models.GetBlockchainCursor(ctx, db, client.Network, networkConfig.StartBlockNum)
	if err != nil {
		return err
	}

	if cursor.LastBlockNum >= latestBlockNum {
		return nil
	}

	processedBlock := cursor.LastBlockNum
	startBlock := cursor.LastBlockNum + 1
	endBlock := latestBlockNum

	if networkConfig.LogBlockRange == 0 {
		return fmt.Errorf("deposit withdraw network %s log block range not configured", client.Network)
	}
	if endBlock-startBlock+1 > networkConfig.LogBlockRange {
		endBlock = startBlock + networkConfig.LogBlockRange - 1
	}
	log.Debugf("Processing ERC20 deposit logs from %d to %d on %s", startBlock, endBlock, client.Network)
	if err := processERC20DepositLogs(ctx, db, client, networkConfig, startBlock, endBlock); err != nil {
		return err
	}
	processedBlock = endBlock

	if err := db.Model(&cursor).Updates(map[string]interface{}{
		"last_block_num":   processedBlock,
		"last_update_time": time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("failed to update blockchain cursor: %w", err)
	}

	return nil
}

var erc20TransferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

func addressTopic(address string) common.Hash {
	return common.BytesToHash(common.HexToAddress(address).Bytes())
}

func processERC20DepositLogs(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient, networkConfig config.DepositWithdrawNetworkConfig, startBlock, endBlock uint64) error {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
		Addresses: []common.Address{common.HexToAddress(networkConfig.Contracts.TokenAddress)},
		Topics: [][]common.Hash{
			{erc20TransferTopic},
			nil,
			{addressTopic(config.GetConfig().RelayAccount.DepositAddress)},
		},
	}
	if err := client.Limiter.Wait(ctx); err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	logs, err := client.RpcClient.FilterLogs(callCtx, query)
	if err != nil {
		return err
	}
	for _, receiptLog := range logs {
		if err := processERC20DepositLog(ctx, db, client, networkConfig, receiptLog); err != nil {
			return err
		}
	}
	return nil
}

func processERC20DepositLog(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient, networkConfig config.DepositWithdrawNetworkConfig, receiptLog types.Log) error {
	if len(receiptLog.Topics) != 3 ||
		receiptLog.Topics[0] != erc20TransferTopic ||
		!strings.EqualFold(receiptLog.Address.Hex(), networkConfig.Contracts.TokenAddress) ||
		receiptLog.Topics[2] != addressTopic(config.GetConfig().RelayAccount.DepositAddress) {
		return nil
	}
	amount := new(big.Int).SetBytes(receiptLog.Data)
	if amount.Sign() <= 0 {
		return nil
	}
	fromAddress := common.BytesToAddress(receiptLog.Topics[1].Bytes()).Hex()

	receipt, err := client.RpcClient.TransactionReceipt(ctx, receiptLog.TxHash)
	if err != nil {
		return err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}
	transfer, err := client.GetTransactionTransfer(ctx, receiptLog.TxHash)
	if err != nil {
		return err
	}
	if !strings.EqualFold(transfer.From.Hex(), fromAddress) {
		return nil
	}
	event, err := models.GetRelayAccountDepositEvent(ctx, db, receiptLog.TxHash.Hex(), client.Network)
	if err != nil {
		return err
	}
	if event != nil {
		return nil
	}
	commitFunc, err := depositRelayAccount(ctx, db, receiptLog.TxHash.Hex(), fromAddress, amount, client.Network)
	if err != nil {
		return err
	}
	if err := commitFunc(); err != nil {
		return err
	}
	log.Infof("Processed ERC20 deposit from %s, amount: %s, tx: %s, network: %s", fromAddress, amount.String(), receiptLog.TxHash.Hex(), client.Network)
	return nil
}

// processBlock processes a single block
func processBlock(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient, blockNum uint64) error {
	header, err := client.RpcClient.HeaderByNumber(ctx, big.NewInt(int64(blockNum)))
	if err != nil {
		return fmt.Errorf("failed to get block %d: %w", blockNum, err)
	}

	txHashes, err := client.GetTransactionHashesFromBlock(ctx, big.NewInt(int64(blockNum)))
	if err != nil {
		return fmt.Errorf("failed to get transaction hashes from block %d: %w", blockNum, err)
	}

	blockTime := time.Unix(int64(header.Time), 0)

	for _, txHashText := range txHashes {
		if !common.IsHexHash(txHashText) {
			return fmt.Errorf("invalid transaction hash %s in block %d", txHashText, blockNum)
		}
		txHash := common.HexToHash(txHashText)
		receipt, err := client.RpcClient.TransactionReceipt(ctx, txHash)
		if err != nil {
			return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", txHash.Hex(), client.Network, err)
		}
		transfer, err := client.GetTransactionTransfer(ctx, txHash)
		if err != nil {
			return fmt.Errorf("failed to get transaction transfer %s, network: %s, error: %w", txHash.Hex(), client.Network, err)
		}
		if err := processTransactionReceiptLogs(ctx, db, receipt, client, blockTime); err != nil {
			log.Errorf("Failed to process transaction receipt logs %s: %v", txHash.Hex(), err)
			return err
		}
		if err := processTransactionTransfer(ctx, db, transfer, receipt, client); err != nil {
			log.Errorf("Failed to process transaction %s: %v", txHash.Hex(), err)
			return err
		}
	}

	return nil
}

func processTransactionReceiptLogs(ctx context.Context, db *gorm.DB, receipt *types.Receipt, client *blockchain.BlockchainClient, blockTime time.Time) error {
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	appConfig := config.GetConfig()
	blockchainCfg, ok := appConfig.Blockchains[client.Network]
	if !ok {
		return fmt.Errorf("network %s not found", client.Network)
	}

	var nodeStakingLogs []*types.Log
	var delegatedStakingLogs []*types.Log
	nodeStakingLogs, delegatedStakingLogs = filterReceiptLogsByContract(
		receipt.Logs,
		blockchainCfg.Contracts.NodeStaking,
		blockchainCfg.Contracts.DelegatedStaking,
	)

	if len(nodeStakingLogs) > 0 {
		if err := processNodeStakingReceiptLogs(nodeStakingLogs, client, nodeStakingReceiptLogHandlers{
			onNodeStaked: func(event *bindings.NodeStakingNodeStaked) error {
				return nodeStaked(ctx, db, event, client.Network)
			},
			onNodeTryUnstaked: func(event *bindings.NodeStakingNodeTryUnstaked) error {
				return nodeTryUnstaked(ctx, db, event, client.Network, blockTime)
			},
			onNodeSlashed: func(event *bindings.NodeStakingNodeSlashed) error {
				return createOrResumeDelegatedSlashJob(ctx, db, event.NodeAddress.Hex(), client.Network)
			},
		}); err != nil {
			return err
		}
	}
	if len(delegatedStakingLogs) > 0 {
		if err := processDelegatedStakingReceiptLogs(ctx, db, delegatedStakingLogs, client); err != nil {
			return err
		}
	}

	return nil
}

func filterReceiptLogsByContract(receiptLogs []*types.Log, nodeStakingAddress, delegatedStakingAddress string) ([]*types.Log, []*types.Log) {
	var nodeStakingLogs []*types.Log
	var delegatedStakingLogs []*types.Log
	for _, receiptLog := range receiptLogs {
		switch {
		case strings.EqualFold(receiptLog.Address.Hex(), nodeStakingAddress):
			nodeStakingLogs = append(nodeStakingLogs, receiptLog)
		case strings.EqualFold(receiptLog.Address.Hex(), delegatedStakingAddress):
			delegatedStakingLogs = append(delegatedStakingLogs, receiptLog)
		}
	}
	return nodeStakingLogs, delegatedStakingLogs
}

func processRelayAccountDepositTransfer(ctx context.Context, db *gorm.DB, transfer *blockchain.TransactionTransfer, receipt *types.Receipt, client *blockchain.BlockchainClient) error {
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	// Check if already processed
	event, err := models.GetRelayAccountDepositEvent(ctx, db, transfer.Hash.Hex(), client.Network)
	if err != nil {
		return err
	}
	if event != nil {
		return nil
	}

	commitFunc, err := depositRelayAccount(ctx, db, transfer.Hash.Hex(), transfer.From.Hex(), transfer.Value, client.Network)
	if err != nil {
		log.Errorf("Failed to process relay account deposit for %s, network: %s, error: %v", transfer.From.Hex(), client.Network, err)
		return err
	}

	if err := commitFunc(); err != nil {
		log.Errorf("Failed to process relay account deposit for %s, network: %s, error: %v", transfer.From.Hex(), client.Network, err)
		return err
	}
	return nil
}

func processNodeStakingTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient, blockTime time.Time) error {
	receipt, err := client.RpcClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	return processNodeStakingReceiptLogs(receipt.Logs, client, nodeStakingReceiptLogHandlers{
		onNodeStaked: func(event *bindings.NodeStakingNodeStaked) error {
			return nodeStaked(ctx, db, event, client.Network)
		},
		onNodeTryUnstaked: func(event *bindings.NodeStakingNodeTryUnstaked) error {
			return nodeTryUnstaked(ctx, db, event, client.Network, blockTime)
		},
		onNodeSlashed: func(event *bindings.NodeStakingNodeSlashed) error {
			return createOrResumeDelegatedSlashJob(ctx, db, event.NodeAddress.Hex(), client.Network)
		},
	})
}

type nodeStakingReceiptLogHandlers struct {
	onNodeStaked      func(*bindings.NodeStakingNodeStaked) error
	onNodeTryUnstaked func(*bindings.NodeStakingNodeTryUnstaked) error
	onNodeSlashed     func(*bindings.NodeStakingNodeSlashed) error
}

func processNodeStakingReceiptLogs(receiptLogs []*types.Log, client *blockchain.BlockchainClient, handlers nodeStakingReceiptLogHandlers) error {
	return processNodeStakingReceiptLogsWithParsers(receiptLogs, nodeStakingReceiptLogParsers{
		parseNodeStaked: func(receiptLog types.Log) (*bindings.NodeStakingNodeStaked, error) {
			return client.NodeStakingContractInstance.ParseNodeStaked(receiptLog)
		},
		parseNodeTryUnstaked: func(receiptLog types.Log) (*bindings.NodeStakingNodeTryUnstaked, error) {
			return client.NodeStakingContractInstance.ParseNodeTryUnstaked(receiptLog)
		},
		parseNodeSlashed: func(receiptLog types.Log) (*bindings.NodeStakingNodeSlashed, error) {
			return client.NodeStakingContractInstance.ParseNodeSlashed(receiptLog)
		},
	}, handlers)
}

type nodeStakingReceiptLogParsers struct {
	parseNodeStaked      func(types.Log) (*bindings.NodeStakingNodeStaked, error)
	parseNodeTryUnstaked func(types.Log) (*bindings.NodeStakingNodeTryUnstaked, error)
	parseNodeSlashed     func(types.Log) (*bindings.NodeStakingNodeSlashed, error)
}

func processNodeStakingReceiptLogsWithParsers(receiptLogs []*types.Log, parsers nodeStakingReceiptLogParsers, handlers nodeStakingReceiptLogHandlers) error {
	for _, receiptLog := range receiptLogs {
		if parsers.parseNodeStaked != nil {
			if event, err := parsers.parseNodeStaked(*receiptLog); err == nil {
				if handlers.onNodeStaked != nil {
					if err := handlers.onNodeStaked(event); err != nil {
						return err
					}
				}
				continue
			}
		}
		if parsers.parseNodeTryUnstaked != nil {
			if event, err := parsers.parseNodeTryUnstaked(*receiptLog); err == nil {
				if handlers.onNodeTryUnstaked != nil {
					if err := handlers.onNodeTryUnstaked(event); err != nil {
						return err
					}
				}
				continue
			}
		}
		if parsers.parseNodeSlashed != nil {
			if event, err := parsers.parseNodeSlashed(*receiptLog); err == nil {
				if handlers.onNodeSlashed != nil {
					if err := handlers.onNodeSlashed(event); err != nil {
						return err
					}
				}
				continue
			}
		}
	}

	return nil
}

func getNodeForChainEvent(ctx context.Context, db *gorm.DB, address, network, eventName string) (*models.Node, error) {
	node, err := models.GetNodeByAddress(ctx, db, address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warnf("%s: skip event for unknown node %s on network %s", eventName, address, network)
			return nil, nil
		}
		return nil, err
	}
	if node.Network != network {
		log.Warnf("%s: skip event for node %s on network %s because Relay node is on network %s", eventName, address, network, node.Network)
		return nil, nil
	}
	return node, nil
}

func nodeStaked(ctx context.Context, db *gorm.DB, event *bindings.NodeStakingNodeStaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	address := event.NodeAddress.Hex()
	stakingAmount := big.NewInt(0).Add(event.StakedBalance, event.StakedCredits)
	node, err := getNodeForChainEvent(dbCtx, db, address, network, "NodeStaked")
	if err != nil {
		log.Errorf("NodeStaked: failed to get node %s: %v", address, err)
		return err
	}
	if node == nil || node.Status == models.NodeStatusQuit {
		return nil
	}
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&node).Update("stake_amount", models.BigInt{Int: *stakingAmount}).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.NodeStakingEvent{NodeAddress: address, StakingAmount: models.BigInt{Int: *stakingAmount}, Network: network}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Errorf("NodeStaked: failed to update node staking for node %s: %v", address, err)
		return err
	}

	// Update value in memory
	totalStakeAmount := new(big.Int).Add(stakingAmount, GetNodeTotalStakeAmount(address, network))
	if totalStakeAmount.Sign() > 0 {
		UpdateMaxStaking(address, totalStakeAmount)
	}

	log.Infof("NodeStaked: successfully updated node %s stake amount to %s",
		address, stakingAmount.String())

	return nil
}

func nodeTryUnstaked(ctx context.Context, db *gorm.DB, event *bindings.NodeStakingNodeTryUnstaked, network string, blockTime time.Time) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dbCancel()

	address := event.NodeAddress.Hex()
	node, err := models.GetNodeByAddress(dbCtx, db, address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("NodeUnstaked: failed to process node unstaked event for node %s: %v", address, err)
		return err
	}
	if node.Network != network {
		return nil
	}

	if blockTime.Before(node.JoinTime) {
		return nil
	}

retryLoop:
	for range 3 {
		switch node.Status {
		case models.NodeStatusAvailable, models.NodeStatusPaused:
			err = SetNodeStatusQuit(dbCtx, db, node, false)
			if err == nil {
				break retryLoop
			} else if errors.Is(err, models.ErrNodeStatusChanged) {
				if err := node.SyncStatus(dbCtx, db); err != nil {
					log.Errorf("NodeUnstaked: failed to process node unstaked event for node %s: %v", address, err)
					return err
				}
				err = nil
			} else {
				log.Errorf("NodeUnstaked: failed to process node unstaked event for node %s: %v", address, err)
				return err
			}
		case models.NodeStatusBusy:
			err = node.Update(dbCtx, db, map[string]interface{}{"status": models.NodeStatusPendingQuit})
			if err == nil {
				break retryLoop
			} else if errors.Is(err, models.ErrNodeStatusChanged) {
				if err := node.SyncStatus(dbCtx, db); err != nil {
					log.Errorf("NodeUnstaked: failed to process node unstaked event for node %s: %v", address, err)
					return err
				}
				err = nil
			} else {
				log.Errorf("NodeUnstaked: failed to process node unstaked event for node %s: %v", address, err)
				return err
			}
		default:
			break retryLoop
		}
	}
	if err != nil {
		log.Errorf("NodeUnstaked: failed to process node unstaked event for node %s: %v", address, err)
		return err
	}
	return nil
}

func processDelegatedStakingTransaction(ctx context.Context, db *gorm.DB, tx *types.Transaction, client *blockchain.BlockchainClient) error {
	receipt, err := client.RpcClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt of %s, network: %s, error: %w", tx.Hash().Hex(), client.Network, err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil
	}

	return processDelegatedStakingReceiptLogs(ctx, db, receipt.Logs, client)
}

func processDelegatedStakingReceiptLogs(ctx context.Context, db *gorm.DB, receiptLogs []*types.Log, client *blockchain.BlockchainClient) error {
	slashedNodes := make(map[string]struct{})
	for _, log := range receiptLogs {
		if event, err := client.DelegatedStakingContractInstance.ParseDelegatorStaked(*log); err == nil {
			if err := updateDelegatedStaking(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
		if event, err := client.DelegatedStakingContractInstance.ParseDelegatorUnstaked(*log); err == nil {
			if err := unstakeDelegatedStaking(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
		if event, err := client.DelegatedStakingContractInstance.ParseNodeDelegatorShareChanged(*log); err == nil {
			if err := changeNodeDelegatorShare(ctx, db, event, client.Network); err != nil {
				return err
			}
			continue
		}
		if event, err := client.DelegatedStakingContractInstance.ParseDelegatorSlashed(*log); err == nil {
			if err := slashDelegatedStaking(ctx, db, event, client.Network); err != nil {
				return err
			}
			slashedNodes[event.NodeAddress.Hex()] = struct{}{}
			continue
		}
	}

	for nodeAddress := range slashedNodes {
		if err := sendNextDelegatedSlashBatch(ctx, db, nodeAddress, client.Network); err != nil {
			return err
		}
	}

	return nil
}

func updateDelegatedStaking(ctx context.Context, db *gorm.DB, event *bindings.DelegatedStakingDelegatorStaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	delegatorAddress := event.DelegatorAddress.Hex()
	nodeAddress := event.NodeAddress.Hex()
	node, err := getNodeForChainEvent(dbCtx, db, nodeAddress, network, "DelegatorStaked")
	if err != nil {
		log.Errorf("UpdateUserStaking: failed to get node %s: %v", nodeAddress, err)
		return err
	}
	if node == nil || node.Status == models.NodeStatusQuit {
		return nil
	}
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userStaking models.Delegation
		if err := tx.Model(&models.Delegation{}).Where("delegator_address = ?", delegatorAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).First(&userStaking).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				userStaking = models.Delegation{
					DelegatorAddress: delegatorAddress,
					NodeAddress:      nodeAddress,
					Amount:           models.BigInt{Int: *event.Amount},
					Valid:            true,
					Network:          network,
				}
			} else {
				return err
			}
		} else {
			userStaking.Amount = models.BigInt{Int: *event.Amount}
			userStaking.Valid = true
		}
		if err := tx.Save(&userStaking).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.DelegatorStakingEvent{
			DelegatorAddress: delegatorAddress,
			NodeAddress:      nodeAddress,
			Amount:           models.BigInt{Int: *event.Amount},
			Network:          network,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Errorf("UpdateUserStaking: failed to update user staking %s -> %s: %v", delegatorAddress, nodeAddress, err)
		return err
	}

	UpdateDelegation(delegatorAddress, nodeAddress, event.Amount, network)
	totalStakeAmount := new(big.Int).Add(&node.StakeAmount.Int, GetNodeTotalStakeAmount(nodeAddress, network))
	UpdateMaxStaking(nodeAddress, totalStakeAmount)

	log.Infof("UpdateUserStaking: successfully updated user %s stake amount to node %s: %s",
		delegatorAddress, nodeAddress, event.Amount.String())

	return nil
}

func unstakeDelegatedStaking(ctx context.Context, db *gorm.DB, event *bindings.DelegatedStakingDelegatorUnstaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	delegatorAddress := event.DelegatorAddress.Hex()
	nodeAddress := event.NodeAddress.Hex()
	node, err := getNodeForChainEvent(dbCtx, db, nodeAddress, network, "DelegatorUnstaked")
	if err != nil {
		log.Errorf("UnstakeUserStaking: failed to get node %s: %v", nodeAddress, err)
		return err
	}
	if node == nil || node.Status == models.NodeStatusQuit {
		return nil
	}

	unstaked := false
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userStaking models.Delegation
		if err := tx.Model(&models.Delegation{}).Where("delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress, nodeAddress, network).First(&userStaking).Error; err != nil {
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
		if err := emitEvent(ctx, tx, &models.DelegatorUnstakingEvent{
			DelegatorAddress: delegatorAddress,
			NodeAddress:      nodeAddress,
			Amount:           userStaking.Amount,
			Network:          network,
		}); err != nil {
			return err
		}
		unstaked = true
		return nil
	}); err != nil {
		log.Errorf("UnstakeUserStaking: failed to unstake user staking %s -> %s: %v", delegatorAddress, nodeAddress, err)
		return err
	}

	if unstaked {
		UnstakeDelegation(delegatorAddress, nodeAddress, network)
		totalStakeAmount := new(big.Int).Add(&node.StakeAmount.Int, GetNodeTotalStakeAmount(nodeAddress, network))
		if totalStakeAmount.Sign() > 0 {
			UpdateMaxStaking(nodeAddress, totalStakeAmount)
		}
	}

	log.Infof("UnstakeUserStaking: successfully unstake user staking %s -> %s",
		delegatorAddress, nodeAddress)

	return nil
}

func changeNodeDelegatorShare(ctx context.Context, db *gorm.DB, event *bindings.DelegatedStakingNodeDelegatorShareChanged, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	nodeAddress := event.NodeAddress.Hex()
	share := event.Share

	node, err := getNodeForChainEvent(dbCtx, db, nodeAddress, network, "NodeDelegatorShareChanged")
	if err != nil {
		log.Errorf("ChangeNodeDelegatorShare: failed to get node %s: %v", nodeAddress, err)
		return err
	}
	if node == nil || node.Status == models.NodeStatusQuit {
		return nil
	}
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Node{}).Where("address = ?", nodeAddress).Where("network = ?", network).Update("delegator_share", share).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.NodeDelegatorShareChangedEvent{
			NodeAddress: nodeAddress,
			Share:       share,
			Network:     network,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Errorf("ChangeNodeDelegatorShare: failed to change delegator share of node %s: %v", nodeAddress, err)
		return err
	}

	SetDelegatorShare(nodeAddress, network, share)
	log.Infof("ChangeNodeDelegatorShare: successfully change delegator share of node %s to %d",
		nodeAddress, share)
	return nil
}

func createOrResumeDelegatedSlashJob(ctx context.Context, db *gorm.DB, nodeAddress, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dbCancel()

	if node, err := getNodeForChainEvent(dbCtx, db, nodeAddress, network, "NodeSlashed"); err != nil {
		log.Errorf("DelegatedSlashJob: failed to get node %s: %v", nodeAddress, err)
		return err
	} else if node == nil {
		return nil
	}
	return sendNextDelegatedSlashBatch(dbCtx, db, nodeAddress, network)
}

func slashDelegatedStaking(ctx context.Context, db *gorm.DB, event *bindings.DelegatedStakingDelegatorSlashed, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	delegatorAddress := event.DelegatorAddress.Hex()
	nodeAddress := event.NodeAddress.Hex()
	var slashed bool
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var job models.DelegatedSlashJob
		jobID := sql.NullInt64{}
		if err := tx.Where("node_address = ? AND network = ?", nodeAddress, network).First(&job).Error; err == nil {
			jobID = sql.NullInt64{Int64: int64(job.ID), Valid: true}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		slashRecord := models.DelegatedStakingSlashRecord{
			SlashJobID:       jobID,
			NodeAddress:      nodeAddress,
			DelegatorAddress: delegatorAddress,
			Network:          network,
			Amount:           models.BigInt{Int: *event.Amount},
			SlashTxHash:      event.Raw.TxHash.Hex(),
			BlockNumber:      event.Raw.BlockNumber,
			LogIndex:         event.Raw.Index,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&slashRecord).Error; err != nil {
			return err
		}

		var delegation models.Delegation
		if err := tx.Where("delegator_address = ? AND node_address = ? AND network = ?", delegatorAddress, nodeAddress, network).First(&delegation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if !delegation.Valid {
			return nil
		}
		if err := tx.Model(&delegation).Update("valid", false).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.DelegatedStakingSlashedEvent{
			DelegatorAddress: delegatorAddress,
			NodeAddress:      nodeAddress,
			Amount:           models.BigInt{Int: *event.Amount},
			Network:          network,
		}); err != nil {
			return err
		}
		slashed = true
		return nil
	}); err != nil {
		log.Errorf("DelegatorSlashed: failed to process delegated slash %s -> %s: %v", delegatorAddress, nodeAddress, err)
		return err
	}

	if slashed {
		UnstakeDelegation(delegatorAddress, nodeAddress, network)
		UpdateMaxStaking(nodeAddress, GetNodeTotalStakeAmount(nodeAddress, network))
	}
	log.Infof("DelegatorSlashed: successfully processed delegated slash %s -> %s", delegatorAddress, nodeAddress)
	return nil
}

func sendNextDelegatedSlashBatch(ctx context.Context, db *gorm.DB, nodeAddress, network string) error {
	appConfig := config.GetConfig()
	blockchainConfig, ok := appConfig.Blockchains[network]
	if !ok {
		return fmt.Errorf("network %s not found", network)
	}
	batchSize := blockchainConfig.DelegatedStakingSlashBatchSize
	if batchSize == 0 {
		return fmt.Errorf("delegated staking slash batch size not configured for network %s", network)
	}

	node := common.HexToAddress(nodeAddress)
	delegatorAddresses, _, err := blockchain.GetNodeStakingInfoPage(ctx, node, network, 1, batchSize)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job models.DelegatedSlashJob
		if err := tx.Where("node_address = ? AND network = ?", nodeAddress, network).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				job = models.DelegatedSlashJob{
					NodeAddress: nodeAddress,
					Network:     network,
					Status:      models.DelegatedSlashJobStatusPending,
				}
				if err := tx.Create(&job).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
		if job.Status == models.DelegatedSlashJobStatusCompleted {
			return nil
		}
		if open, err := hasOpenDelegatedSlashBatchTransaction(ctx, tx, &job); err != nil {
			return err
		} else if open {
			return nil
		}
		if len(delegatorAddresses) == 0 {
			return tx.Model(&job).Updates(map[string]interface{}{
				"status": models.DelegatedSlashJobStatusCompleted,
			}).Error
		}

		blockchainTransaction, err := blockchain.SlashNodeDelegations(ctx, tx, node, delegatorAddresses, network)
		if err != nil {
			return err
		}
		return tx.Model(&job).Updates(map[string]interface{}{
			"status":                      models.DelegatedSlashJobStatusProcessing,
			"latest_batch_transaction_id": sql.NullInt64{Int64: int64(blockchainTransaction.ID), Valid: true},
			"last_error":                  sql.NullString{},
		}).Error
	})
}

func hasOpenDelegatedSlashBatchTransaction(ctx context.Context, db *gorm.DB, job *models.DelegatedSlashJob) (bool, error) {
	if !job.LatestBatchTransactionID.Valid {
		return false, nil
	}
	transaction, err := models.GetTransactionByID(ctx, db, uint(job.LatestBatchTransactionID.Int64))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if transaction.Status == models.TransactionStatusPending || transaction.Status == models.TransactionStatusSent {
		return true, nil
	}
	retryTransactions, err := models.GetRetryTransactionsByID(ctx, db, transaction.ID)
	if err != nil {
		return false, err
	}
	for _, retryTransaction := range retryTransactions {
		if retryTransaction.Status == models.TransactionStatusPending || retryTransaction.Status == models.TransactionStatusSent {
			return true, nil
		}
	}
	return false, nil
}

// processTransactionTransfer processes a transaction using raw RPC transfer fields.
func processTransactionTransfer(ctx context.Context, db *gorm.DB, transfer *blockchain.TransactionTransfer, receipt *types.Receipt, client *blockchain.BlockchainClient) error {
	appConfig := config.GetConfig()
	if !isRelayAccountDepositTransfer(transfer, appConfig.RelayAccount.DepositAddress) {
		return nil
	}

	return processRelayAccountDepositTransfer(ctx, db, transfer, receipt, client)
}

func isRelayAccountDepositTransfer(transfer *blockchain.TransactionTransfer, depositAddress string) bool {
	return transfer.To != nil &&
		transfer.Value != nil &&
		transfer.Value.Sign() > 0 &&
		len(transfer.Input) == 0 &&
		strings.EqualFold(transfer.To.Hex(), depositAddress)
}
