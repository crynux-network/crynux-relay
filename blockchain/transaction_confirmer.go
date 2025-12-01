package blockchain

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TransactionConfirmer confirms the status of sent transactions
type TransactionConfirmer struct {
	db            *gorm.DB
	processingTxs sync.Map
	txQueue       chan *models.BlockchainTransaction
	stopChan      chan struct{}
	isRunning     bool
	batchSize     int
	pollInterval  time.Duration
	limiter       chan struct{}
}

// NewTransactionConfirmer creates a new transaction confirmer instance
func NewTransactionConfirmer(db *gorm.DB) *TransactionConfirmer {
	return &TransactionConfirmer{
		db:           db,
		txQueue:      make(chan *models.BlockchainTransaction, 100),
		stopChan:     make(chan struct{}),
		isRunning:    false,
		batchSize:    50,
		pollInterval: 5 * time.Second,
		limiter:      make(chan struct{}, 10),
	}
}

// Start starts the transaction confirmer goroutine
func (tc *TransactionConfirmer) Start(ctx context.Context) {
	if tc.isRunning {
		return
	}

	tc.isRunning = true
	go tc.run(ctx)
	log.Info("Transaction confirmer started")
}

// Stop stops the transaction confirmer goroutine
func (tc *TransactionConfirmer) Stop() {
	if !tc.isRunning {
		return
	}

	close(tc.stopChan)
	tc.isRunning = false
	log.Info("Transaction confirmer stopped")
}

// run is the main loop for confirming transactions
func (tc *TransactionConfirmer) run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go tc.getSentTransactions(ctx)
	go tc.processSentTransactions(ctx)

	select {
	case <-tc.stopChan:
		close(tc.txQueue)
		return
	case <-ctx.Done():
		close(tc.txQueue)
		return
	}
}

// processSentTransactions processes a batch of sent transactions for confirmation
func (tc *TransactionConfirmer) getSentTransactions(ctx context.Context) {
	ticker := time.NewTicker(tc.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get sent transactions that need confirmation
			transactions, err := func(ctx context.Context) ([]models.BlockchainTransaction, error) {
				var allTransactions []models.BlockchainTransaction
				offset := 0
				for {
					transactions, err := models.GetSentTransactions(ctx, tc.db, offset, tc.batchSize)
					if err != nil {
						return nil, err
					}

					if len(transactions) == 0 {
						break
					}
					allTransactions = append(allTransactions, transactions...)
					offset += len(transactions)
				}
				return allTransactions, nil
			}(ctx)
			if err != nil {
				log.Errorf("Error getting sent transactions: %v", err)
				continue
			}
			if len(transactions) == 0 {
				continue
			}
			var cnt int
			for _, transaction := range transactions {
				_, loaded := tc.processingTxs.LoadOrStore(transaction.ID, struct{}{})
				if !loaded {
					select {
					case <-ctx.Done():
						return
					case tc.txQueue <- &transaction:
						cnt++
					}
				}
			}
			log.Infof("Processing %d sent transactions for confirmation", cnt)
		}
	}
}

func (tc *TransactionConfirmer) processSentTransactions(ctx context.Context) {
	for transaction := range tc.txQueue {
		tc.limiter <- struct{}{}
		go func() {
			defer func() {
				<-tc.limiter
			}()
			if err := tc.confirmTransaction(ctx, transaction); err != nil {
				log.Errorf("Failed to confirm transaction %d: %v", transaction.ID, err)
			}
		}()
	}
}

// confirmTransaction confirms the status of a single transaction
func (tc *TransactionConfirmer) confirmTransaction(ctx context.Context, transaction *models.BlockchainTransaction) error {
	defer func() {
		tc.processingTxs.Delete(transaction.ID)
	}()

	if !transaction.TxHash.Valid {
		log.Warnf("Transaction %d has no tx hash", transaction.ID)
		return nil
	}
	if !transaction.SentAt.Valid {
		log.Warnf("Transaction %d has no sent at", transaction.ID)
		return nil
	}

	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[transaction.Network]
	if !ok {
		return fmt.Errorf("network %s not found", transaction.Network)
	}
	client, err := GetBlockchainClient(transaction.Network)
	if err != nil {
		log.Errorf("Error getting blockchain client: %v", err)
		return err
	}
	
	waitDeadline := transaction.SentAt.Time.Add(time.Duration(blockchain.ReceiptWaitTime) * time.Second)
	if time.Now().After(waitDeadline) {
		log.Warnf("Transaction %d has waited too long for receipt", transaction.ID)
		if err := tc.handleTimedOutTransaction(ctx, client, transaction); err != nil {
			log.Errorf("Failed to handle timed out transaction: %v", err)
			return err
		}
		return nil
	}
	
	txHash := common.HexToHash(transaction.TxHash.String)
	
	// Get transaction receipt
	receipt, err := client.RpcClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		// If transaction is not found, it might still be pending
		if errors.Is(err, ethereum.NotFound) {
			log.Debugf("Transaction %s is still pending", txHash.Hex())
			return nil
		}

		// If there's a timeout or other error, mark as failed
		log.Errorf("Error getting receipt for transaction %s: %v", txHash.Hex(), err)
		return err
	}

	// Check transaction status
	if receipt.Status == types.ReceiptStatusSuccessful {
		// Transaction successful
		if err := tc.handleSuccessfulTransaction(ctx, transaction, receipt); err != nil {
			log.Errorf("Failed to handle successful transaction: %v", err)
			return err
		}
	} else {
		// Transaction failed
		if err := tc.handleFailedTransaction(ctx, client, transaction, receipt); err != nil {
			log.Errorf("Failed to handle failed transaction: %v", err)
			return err
		}
	}

	return nil
}

// handleSuccessfulTransaction handles a successful transaction
func (tc *TransactionConfirmer) handleSuccessfulTransaction(ctx context.Context, transaction *models.BlockchainTransaction, receipt *types.Receipt) error {
	// Update transaction with receipt information
	if err := transaction.MarkConfirmed(ctx, tc.db, receipt.BlockNumber.Int64(), int64(receipt.GasUsed), receipt.EffectiveGasPrice.String()); err != nil {
		return err
	}

	log.Infof("Transaction %d confirmed successfully in block %d", transaction.ID, receipt.BlockNumber.Int64())
	return nil
}

// handleFailedTransaction handles a failed transaction
func (tc *TransactionConfirmer) handleFailedTransaction(ctx context.Context, client *BlockchainClient, transaction *models.BlockchainTransaction, receipt *types.Receipt) error {
	// Get error message from receipt
	errorMsg, err := client.GetErrorMessageFromReceipt(ctx, receipt)
	if err != nil {
		errorMsg = fmt.Sprintf("Transaction failed with status 0: %v", err)
	}

	// Update transaction with receipt information
	if err := tc.db.Transaction(func(tx *gorm.DB) error {
		if err := transaction.MarkFailed(ctx, tx, receipt.BlockNumber.Int64(), int64(receipt.GasUsed), receipt.EffectiveGasPrice.String(), errorMsg); err != nil {
			return err
		}
		if transaction.RetryCount < transaction.MaxRetries {
			if err := transaction.CreateRetryTransaction(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	log.Infof("Transaction %d failed, will retry (attempt %d/%d)", transaction.ID, transaction.RetryCount+1, transaction.MaxRetries)

	return nil
}

func (tc *TransactionConfirmer) handleTimedOutTransaction(ctx context.Context, client *BlockchainClient, transaction *models.BlockchainTransaction) error {
	if err := tc.db.Transaction(func(tx *gorm.DB) error {
		if err := transaction.MarkFailed(ctx, tx, 0, 0, "", "Transaction timed out"); err != nil {
			return err
		}
		if transaction.RetryCount < transaction.MaxRetries {
			if err := transaction.CreateRetryTransaction(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	client.resetNonce()
	log.Infof("Transaction %d timed out, will retry (attempt %d/%d)", transaction.ID, transaction.RetryCount+1, transaction.MaxRetries)

	return nil
}
