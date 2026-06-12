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

type erc20DepositLogData struct {
	fromAddress string
	amount      *big.Int
}

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
	if len(logs) > 0 {
		log.Infof("ERC20 deposit log scan result on %s from block %d to %d: %d log(s), token: %s, deposit address: %s",
			client.Network, startBlock, endBlock, len(logs), networkConfig.Contracts.TokenAddress, config.GetConfig().RelayAccount.DepositAddress)
	}
	for _, receiptLog := range logs {
		if err := processERC20DepositLog(ctx, db, client, networkConfig, receiptLog); err != nil {
			return err
		}
	}
	return nil
}

func parseERC20DepositLog(network, tokenAddress, depositAddress string, receiptLog types.Log) (*erc20DepositLogData, bool) {
	if len(receiptLog.Topics) != 3 ||
		receiptLog.Topics[0] != erc20TransferTopic ||
		!strings.EqualFold(receiptLog.Address.Hex(), tokenAddress) ||
		receiptLog.Topics[2] != addressTopic(depositAddress) {
		log.Errorf("Skipping unmatched ERC20 deposit log on %s, tx: %s, block: %d, log index: %d, contract: %s, topics: %d",
			network, receiptLog.TxHash.Hex(), receiptLog.BlockNumber, receiptLog.Index, receiptLog.Address.Hex(), len(receiptLog.Topics))
		return nil, false
	}
	amount := new(big.Int).SetBytes(receiptLog.Data)
	if amount.Sign() <= 0 {
		log.Errorf("Skipping zero ERC20 deposit log on %s, tx: %s, block: %d, log index: %d", network, receiptLog.TxHash.Hex(), receiptLog.BlockNumber, receiptLog.Index)
		return nil, false
	}
	fromAddress := common.BytesToAddress(receiptLog.Topics[1].Bytes()).Hex()
	if fromAddress == (common.Address{}).Hex() {
		log.Errorf("Skipping ERC20 deposit log with zero from address on %s, tx: %s, block: %d, log index: %d, amount: %s",
			network, receiptLog.TxHash.Hex(), receiptLog.BlockNumber, receiptLog.Index, amount.String())
		return nil, false
	}
	log.Infof("Matched ERC20 deposit log on %s, tx: %s, block: %d, log index: %d, from: %s, amount: %s",
		network, receiptLog.TxHash.Hex(), receiptLog.BlockNumber, receiptLog.Index, fromAddress, amount.String())
	return &erc20DepositLogData{
		fromAddress: fromAddress,
		amount:      amount,
	}, true
}

func processERC20DepositLog(ctx context.Context, db *gorm.DB, client *blockchain.BlockchainClient, networkConfig config.DepositWithdrawNetworkConfig, receiptLog types.Log) error {
	depositLog, ok := parseERC20DepositLog(client.Network, networkConfig.Contracts.TokenAddress, config.GetConfig().RelayAccount.DepositAddress, receiptLog)
	if !ok {
		return nil
	}

	receipt, err := client.RpcClient.TransactionReceipt(ctx, receiptLog.TxHash)
	if err != nil {
		return err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		log.Errorf("Skipping ERC20 deposit log with unsuccessful receipt on %s, tx: %s, receipt status: %d", client.Network, receiptLog.TxHash.Hex(), receipt.Status)
		return nil
	}
	event, err := models.GetRelayAccountDepositEvent(ctx, db, receiptLog.TxHash.Hex(), client.Network)
	if err != nil {
		return err
	}
	if event != nil {
		log.Errorf("Skipping already processed ERC20 deposit on %s, tx: %s, existing event id: %d", client.Network, receiptLog.TxHash.Hex(), event.ID)
		return nil
	}
	commitFunc, err := depositRelayAccount(ctx, db, receiptLog.TxHash.Hex(), depositLog.fromAddress, depositLog.amount, client.Network)
	if err != nil {
		return err
	}
	if err := commitFunc(); err != nil {
		return err
	}
	log.Infof("Processed ERC20 deposit from %s, amount: %s, tx: %s, network: %s", depositLog.fromAddress, depositLog.amount.String(), receiptLog.TxHash.Hex(), client.Network)
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
				return createOrResumeDelegatedSlashJob(ctx, db, event, client.Network)
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
		log.Errorf("Skipping native deposit with unsuccessful receipt on %s, tx: %s, receipt status: %d", client.Network, transfer.Hash.Hex(), receipt.Status)
		return nil
	}

	// Check if already processed
	event, err := models.GetRelayAccountDepositEvent(ctx, db, transfer.Hash.Hex(), client.Network)
	if err != nil {
		return err
	}
	if event != nil {
		log.Errorf("Skipping already processed native deposit on %s, tx: %s, existing event id: %d", client.Network, transfer.Hash.Hex(), event.ID)
		return nil
	}

	log.Infof("Processing native deposit on %s, tx: %s, from: %s, to: %s, amount: %s",
		client.Network, transfer.Hash.Hex(), transfer.From.Hex(), transfer.To.Hex(), transfer.Value.String())
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
			return createOrResumeDelegatedSlashJob(ctx, db, event, client.Network)
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

func getNodeForDelegationEvent(ctx context.Context, db *gorm.DB, address, network, eventName string) (*models.Node, error) {
	node, err := models.GetNodeByAddress(ctx, db, address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warnf("%s: skip delegation event for unknown node %s on network %s", eventName, address, network)
			return nil, nil
		}
		return nil, err
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
	node.StakeAmount = models.BigInt{Int: *stakingAmount}
	totalStakeAmount := GetNodeScoreStakeAmount(*node, time.Now().UTC())
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
		job, err := models.GetUnfinishedDelegatedSlashJob(ctx, db, nodeAddress, client.Network)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if err := sendNextDelegatedSlashBatch(ctx, db, *job); err != nil {
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
	node, err := getNodeForDelegationEvent(dbCtx, db, nodeAddress, network, "DelegatorStaked")
	if err != nil {
		log.Errorf("UpdateUserStaking: failed to get node %s: %v", nodeAddress, err)
		return err
	}
	if node == nil {
		return nil
	}
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		userStaking := models.Delegation{
			DelegatorAddress: delegatorAddress,
			NodeAddress:      nodeAddress,
			Amount:           models.BigInt{Int: *event.Amount},
			Slashed:          false,
			Network:          network,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "delegator_address"}, {Name: "node_address"}, {Name: "network"}},
			DoUpdates: clause.AssignmentColumns([]string{"amount", "slashed", "updated_at"}),
		}).Create(&userStaking).Error; err != nil {
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
	if node.Network == network {
		UpdateMaxStaking(nodeAddress, GetNodeScoreStakeAmount(*node, time.Now().UTC()))
	}

	log.Infof("UpdateUserStaking: successfully updated user %s stake amount to node %s: %s",
		delegatorAddress, nodeAddress, event.Amount.String())

	return nil
}

func unstakeDelegatedStaking(ctx context.Context, db *gorm.DB, event *bindings.DelegatedStakingDelegatorUnstaked, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()

	delegatorAddress := event.DelegatorAddress.Hex()
	nodeAddress := event.NodeAddress.Hex()
	node, err := getNodeForDelegationEvent(dbCtx, db, nodeAddress, network, "DelegatorUnstaked")
	if err != nil {
		log.Errorf("UnstakeUserStaking: failed to get node %s: %v", nodeAddress, err)
		return err
	}
	if node == nil {
		return nil
	}

	unstaked := false
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userStaking models.Delegation
		if err := tx.Model(&models.Delegation{}).Where("delegator_address = ? AND node_address = ? AND network = ? AND slashed = ?", delegatorAddress, nodeAddress, network, false).First(&userStaking).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		if err := tx.Unscoped().Delete(&userStaking).Error; err != nil {
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
		if node.Network == network {
			totalStakeAmount := GetNodeScoreStakeAmount(*node, time.Now().UTC())
			if totalStakeAmount.Sign() > 0 {
				UpdateMaxStaking(nodeAddress, totalStakeAmount)
			}
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

func createOrResumeDelegatedSlashJob(ctx context.Context, db *gorm.DB, event *bindings.NodeStakingNodeSlashed, network string) error {
	dbCtx, dbCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dbCancel()

	nodeAddress := event.NodeAddress.Hex()
	if node, err := getNodeForChainEvent(dbCtx, db, nodeAddress, network, "NodeSlashed"); err != nil {
		log.Errorf("DelegatedSlashJob: failed to get node %s: %v", nodeAddress, err)
		return err
	} else if node == nil {
		return nil
	}
	if err := SlashNodeVestings(ctx, db, nodeAddress, time.Now().UTC()); err != nil {
		return err
	}
	job, err := getOrCreateDelegatedSlashJob(dbCtx, db, event, network)
	if err != nil {
		return err
	}
	return sendNextDelegatedSlashBatch(dbCtx, db, *job)
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
		if err := tx.Where("node_address = ? AND network = ? AND status <> ?", nodeAddress, network, models.DelegatedSlashJobStatusCompleted).Order("id DESC").First(&job).Error; err == nil {
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
		if err := tx.Where("delegator_address = ? AND node_address = ? AND network = ? AND slashed = ?", delegatorAddress, nodeAddress, network, false).First(&delegation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("missing current delegation for delegated slash %s -> %s on %s", delegatorAddress, nodeAddress, network)
			}
			return err
		}
		if err := tx.Model(&delegation).Updates(map[string]interface{}{
			"amount":  models.BigInt{Int: *event.Amount},
			"slashed": true,
		}).Error; err != nil {
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
		if node, err := models.GetNodeByAddress(ctx, db, nodeAddress); err == nil {
			if node.Network == network {
				UpdateMaxStaking(nodeAddress, GetNodeScoreStakeAmount(*node, time.Now().UTC()))
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			UpdateMaxStaking(nodeAddress, big.NewInt(0))
		} else {
			return err
		}
	}
	log.Infof("DelegatorSlashed: successfully processed delegated slash %s -> %s", delegatorAddress, nodeAddress)
	return nil
}

func getOrCreateDelegatedSlashJob(ctx context.Context, db *gorm.DB, event *bindings.NodeStakingNodeSlashed, network string) (*models.DelegatedSlashJob, error) {
	nodeAddress := event.NodeAddress.Hex()
	nodeSlashTxHash := event.Raw.TxHash.Hex()
	nodeSlashLogIndex := int64(event.Raw.Index)
	if event.Raw.TxHash != (common.Hash{}) {
		var eventJob models.DelegatedSlashJob
		err := db.WithContext(ctx).
			Where("network = ? AND node_slash_tx_hash = ? AND node_slash_log_index = ?", network, nodeSlashTxHash, nodeSlashLogIndex).
			First(&eventJob).Error
		if err == nil {
			return &eventJob, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	var job models.DelegatedSlashJob
	if err := db.WithContext(ctx).Where("node_address = ? AND network = ? AND status <> ?", nodeAddress, network, models.DelegatedSlashJobStatusCompleted).Order("id DESC").First(&job).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		job = models.DelegatedSlashJob{
			NodeAddress:       nodeAddress,
			Network:           network,
			Status:            models.DelegatedSlashJobStatusPending,
			NodeSlashTxHash:   sql.NullString{String: nodeSlashTxHash, Valid: event.Raw.TxHash != (common.Hash{})},
			NodeSlashLogIndex: sql.NullInt64{Int64: nodeSlashLogIndex, Valid: event.Raw.TxHash != (common.Hash{})},
		}
		if err := db.WithContext(ctx).Create(&job).Error; err != nil {
			return nil, err
		}
	}
	return &job, nil
}

func sendNextDelegatedSlashBatch(ctx context.Context, db *gorm.DB, job models.DelegatedSlashJob) error {
	appConfig := config.GetConfig()
	blockchainConfig, ok := appConfig.Blockchains[job.Network]
	if !ok {
		return fmt.Errorf("network %s not found", job.Network)
	}
	batchSize := blockchainConfig.DelegatedStakingSlashBatchSize
	if batchSize == 0 {
		return fmt.Errorf("delegated staking slash batch size not configured for network %s", job.Network)
	}

	node := common.HexToAddress(job.NodeAddress)
	delegatorAddresses, _, err := blockchain.GetNodeStakingInfoPage(ctx, node, job.Network, 1, batchSize)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currentJob models.DelegatedSlashJob
		if err := tx.First(&currentJob, job.ID).Error; err != nil {
			return err
		}
		if currentJob.Status == models.DelegatedSlashJobStatusCompleted {
			return nil
		}
		if open, err := hasOpenDelegatedSlashBatchTransaction(ctx, tx, &currentJob); err != nil {
			return err
		} else if open {
			return nil
		}
		if len(delegatorAddresses) == 0 {
			return tx.Model(&currentJob).Updates(map[string]interface{}{
				"status": models.DelegatedSlashJobStatusCompleted,
			}).Error
		}

		blockchainTransaction, err := blockchain.SlashNodeDelegations(ctx, tx, node, delegatorAddresses, job.Network)
		if err != nil {
			return err
		}
		return tx.Model(&currentJob).Updates(map[string]interface{}{
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
	if !isTransferToRelayAccountDepositAddress(transfer, appConfig.RelayAccount.DepositAddress) {
		return nil
	}
	if !isRelayAccountDepositTransfer(transfer, appConfig.RelayAccount.DepositAddress) {
		log.Errorf("Skipping invalid native deposit transfer on %s, tx: %s, from: %s, to: %s, value: %s, input bytes: %d",
			client.Network, transfer.Hash.Hex(), transfer.From.Hex(), transfer.To.Hex(), nativeTransferValueText(transfer), len(transfer.Input))
		return nil
	}

	return processRelayAccountDepositTransfer(ctx, db, transfer, receipt, client)
}

func isTransferToRelayAccountDepositAddress(transfer *blockchain.TransactionTransfer, depositAddress string) bool {
	return transfer.To != nil && strings.EqualFold(transfer.To.Hex(), depositAddress)
}

func nativeTransferValueText(transfer *blockchain.TransactionTransfer) string {
	if transfer.Value == nil {
		return "<nil>"
	}
	return transfer.Value.String()
}

func isRelayAccountDepositTransfer(transfer *blockchain.TransactionTransfer, depositAddress string) bool {
	return isTransferToRelayAccountDepositAddress(transfer, depositAddress) &&
		transfer.Value != nil &&
		transfer.Value.Sign() > 0 &&
		len(transfer.Input) == 0
}
