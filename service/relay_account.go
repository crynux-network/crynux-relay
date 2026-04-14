package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var ErrInsufficientRelayAccount = errors.New("insufficient relay account balance")

type relayAccountCacheType struct {
	accounts map[string]*big.Int
	mu       sync.RWMutex
}

var relayAccountCache = &relayAccountCacheType{
	accounts: make(map[string]*big.Int),
}

func splitRelayAccountEventReason(eventType models.RelayAccountEventType, reason string) ([]string, bool) {
	eventTypeStr := fmt.Sprintf("%d", eventType)
	if eventType == models.RelayAccountEventTypeDeposit || eventType == models.RelayAccountEventTypeUserDelegation {
		parts := strings.SplitN(reason, "-", 3)
		if len(parts) != 3 || parts[0] != eventTypeStr {
			return nil, false
		}
		return parts, true
	}
	parts := strings.SplitN(reason, "-", 2)
	if len(parts) != 2 || parts[0] != eventTypeStr {
		return nil, false
	}
	return parts, true
}

func InitRelayAccountCache(ctx context.Context, db *gorm.DB) error {
	for {
		events, err := getPendingRelayAccountEvents(ctx, db, 50)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			break
		}
		if err := processPendingRelayAccountEvents(ctx, db, events); err != nil {
			return err
		}
	}
	return nil
}

func StartRelayAccountSync(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syncRelayAccountsToDB(ctx, db); err != nil {
				log.Errorf("Failed to sync relay accounts: %v", err)
			}
		}
	}
}

func getPendingRelayAccountEvents(ctx context.Context, db *gorm.DB, limit int) ([]models.RelayAccountEvent, error) {
	var events []models.RelayAccountEvent
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := db.WithContext(dbCtx).
		Where("status = ?", models.RelayAccountEventStatusPending).
		Order("id").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func validatePendingRelayAccountEvents(ctx context.Context, db *gorm.DB, events []models.RelayAccountEvent) ([]models.RelayAccountEvent, []models.RelayAccountEvent, error) {
	invalidEvents := make([]models.RelayAccountEvent, 0)
	candidateEvents := make([]models.RelayAccountEvent, 0)
	taskIDSet := make(map[string]struct{})
	withdrawIDSet := make(map[uint]struct{})
	balanceAddressSet := make(map[string]struct{})
	depositReasonByEventID := make(map[uint][2]string)

	for _, event := range events {
		reasons, ok := splitRelayAccountEventReason(event.Type, event.Reason)
		if !ok {
			invalidEvents = append(invalidEvents, event)
			continue
		}
		switch event.Type {
		case models.RelayAccountEventTypeDeposit:
			depositReasonByEventID[event.ID] = [2]string{reasons[1], reasons[2]}
		case models.RelayAccountEventTypeTaskPayment, models.RelayAccountEventTypeTaskIncome, models.RelayAccountEventTypeDaoTaskShare, models.RelayAccountEventTypeTaskRefund, models.RelayAccountEventTypeUserDelegation:
			taskIDSet[reasons[1]] = struct{}{}
			if event.Type == models.RelayAccountEventTypeTaskPayment {
				balanceAddressSet[event.Address] = struct{}{}
			}
		case models.RelayAccountEventTypeWithdraw, models.RelayAccountEventTypeWithdrawRefund, models.RelayAccountEventTypeWithdrawFeeIncome:
			withdrawID, err := strconv.ParseUint(reasons[1], 10, 64)
			if err != nil {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			withdrawIDSet[uint(withdrawID)] = struct{}{}
			if event.Type == models.RelayAccountEventTypeWithdraw {
				balanceAddressSet[event.Address] = struct{}{}
			}
		default:
			invalidEvents = append(invalidEvents, event)
			continue
		}
		candidateEvents = append(candidateEvents, event)
	}

	taskIDs := make([]string, 0, len(taskIDSet))
	for taskID := range taskIDSet {
		taskIDs = append(taskIDs, taskID)
	}
	withdrawIDs := make([]uint, 0, len(withdrawIDSet))
	for withdrawID := range withdrawIDSet {
		withdrawIDs = append(withdrawIDs, withdrawID)
	}
	balanceAddresses := make([]string, 0, len(balanceAddressSet))
	for address := range balanceAddressSet {
		balanceAddresses = append(balanceAddresses, address)
	}

	var tasks []models.InferenceTask
	if len(taskIDs) > 0 {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.WithContext(dbCtx).
			Where("task_id_commitment IN (?)", taskIDs).
			Find(&tasks).Error; err != nil {
			return nil, nil, err
		}
	}
	taskMap := make(map[string]models.InferenceTask)
	for _, task := range tasks {
		taskMap[task.TaskIDCommitment] = task
	}

	var withdrawRecords []models.WithdrawRecord
	if len(withdrawIDs) > 0 {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.WithContext(dbCtx).
			Where("id IN (?)", withdrawIDs).
			Find(&withdrawRecords).Error; err != nil {
			return nil, nil, err
		}
	}
	withdrawMap := make(map[uint]models.WithdrawRecord)
	for _, record := range withdrawRecords {
		withdrawMap[record.ID] = record
	}

	var relayAccounts []models.RelayAccount
	if len(balanceAddresses) > 0 {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.WithContext(dbCtx).
			Where("address IN (?)", balanceAddresses).
			Find(&relayAccounts).Error; err != nil {
			return nil, nil, err
		}
	}
	relayBalanceMap := make(map[string]*big.Int, len(balanceAddresses))
	for _, address := range balanceAddresses {
		relayBalanceMap[address] = big.NewInt(0)
	}
	for _, account := range relayAccounts {
		relayBalanceMap[account.Address] = new(big.Int).Set(&account.Balance.Int)
	}

	type depositTxData struct {
		FromAddress string
		Amount      *big.Int
	}
	validDepositTxs := make(map[string]depositTxData)
	appConfig := config.GetConfig()
	for _, depositReason := range depositReasonByEventID {
		txHash := depositReason[0]
		network := depositReason[1]
		key := fmt.Sprintf("%s-%s", network, txHash)
		if _, exists := validDepositTxs[key]; exists {
			continue
		}
		client, err := blockchain.GetBlockchainClient(network)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := client.RpcClient.TransactionReceipt(ctx, common.HexToHash(txHash))
		if errors.Is(err, ethereum.NotFound) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			continue
		}
		tx, _, err := client.RpcClient.TransactionByHash(ctx, common.HexToHash(txHash))
		if errors.Is(err, ethereum.NotFound) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if tx.To() == nil || !strings.EqualFold(tx.To().Hex(), appConfig.RelayAccount.DepositAddress) {
			continue
		}
		from, err := types.Sender(types.LatestSignerForChainID(client.ChainID), tx)
		if err != nil {
			continue
		}
		validDepositTxs[key] = depositTxData{
			FromAddress: from.Hex(),
			Amount:      tx.Value(),
		}
	}

	validEvents := make([]models.RelayAccountEvent, 0, len(candidateEvents))
	for _, event := range candidateEvents {
		reasons, ok := splitRelayAccountEventReason(event.Type, event.Reason)
		if !ok {
			invalidEvents = append(invalidEvents, event)
			continue
		}
		switch event.Type {
		case models.RelayAccountEventTypeDeposit:
			key := fmt.Sprintf("%s-%s", reasons[2], reasons[1])
			depositTx, ok := validDepositTxs[key]
			if !ok || !strings.EqualFold(depositTx.FromAddress, event.Address) || depositTx.Amount.Cmp(&event.Amount.Int) != 0 {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			balance := relayBalanceMap[event.Address]
			if balance != nil {
				balance.Add(balance, &event.Amount.Int)
			}
			validEvents = append(validEvents, event)
		case models.RelayAccountEventTypeTaskPayment:
			task, ok := taskMap[reasons[1]]
			if !ok || event.Amount.Int.Cmp(&task.TaskFee.Int) != 0 {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			balance := relayBalanceMap[event.Address]
			if balance != nil && balance.Cmp(&event.Amount.Int) < 0 {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			if balance != nil {
				balance.Sub(balance, &event.Amount.Int)
			}
			validEvents = append(validEvents, event)
		case models.RelayAccountEventTypeTaskIncome, models.RelayAccountEventTypeDaoTaskShare, models.RelayAccountEventTypeUserDelegation:
			task, ok := taskMap[reasons[1]]
			if !ok {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			if task.Status != models.TaskEndSuccess && task.Status != models.TaskEndGroupSuccess && task.Status != models.TaskEndGroupRefund {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			balance := relayBalanceMap[event.Address]
			if balance != nil {
				balance.Add(balance, &event.Amount.Int)
			}
			validEvents = append(validEvents, event)
		case models.RelayAccountEventTypeTaskRefund:
			task, ok := taskMap[reasons[1]]
			if !ok || event.Amount.Int.Cmp(&task.TaskFee.Int) != 0 {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			if task.Status != models.TaskEndGroupRefund && task.Status != models.TaskEndAborted {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			balance := relayBalanceMap[event.Address]
			if balance != nil {
				balance.Add(balance, &event.Amount.Int)
			}
			validEvents = append(validEvents, event)
		case models.RelayAccountEventTypeWithdraw:
			withdrawID, _ := strconv.ParseUint(reasons[1], 10, 64)
			if _, ok := withdrawMap[uint(withdrawID)]; !ok {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			balance := relayBalanceMap[event.Address]
			if balance != nil && balance.Cmp(&event.Amount.Int) < 0 {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			if balance != nil {
				balance.Sub(balance, &event.Amount.Int)
			}
			validEvents = append(validEvents, event)
		case models.RelayAccountEventTypeWithdrawRefund, models.RelayAccountEventTypeWithdrawFeeIncome:
			withdrawID, _ := strconv.ParseUint(reasons[1], 10, 64)
			if _, ok := withdrawMap[uint(withdrawID)]; ok {
				balance := relayBalanceMap[event.Address]
				if balance != nil {
					balance.Add(balance, &event.Amount.Int)
				}
				validEvents = append(validEvents, event)
			} else {
				invalidEvents = append(invalidEvents, event)
			}
		}
	}

	return validEvents, invalidEvents, nil
}

func mergeRelayAccountEvents(events []models.RelayAccountEvent) map[string]*big.Int {
	merged := make(map[string]*big.Int)
	for _, event := range events {
		if _, exists := merged[event.Address]; !exists {
			merged[event.Address] = big.NewInt(0)
		}
		switch event.Type {
		case models.RelayAccountEventTypeTaskPayment, models.RelayAccountEventTypeWithdraw:
			merged[event.Address].Sub(merged[event.Address], &event.Amount.Int)
		default:
			merged[event.Address].Add(merged[event.Address], &event.Amount.Int)
		}
	}
	return merged
}

func processPendingRelayAccountEvents(ctx context.Context, db *gorm.DB, events []models.RelayAccountEvent) error {
	validEvents, invalidEvents, err := validatePendingRelayAccountEvents(ctx, db, events)
	if err != nil {
		return err
	}
	dbWithCtx := db.WithContext(ctx)

	if len(invalidEvents) > 0 {
		invalidIDs := make([]uint, 0, len(invalidEvents))
		invalidWithdrawEventIDs := make([]uint, 0)
		invalidDepositEventIDs := make([]uint, 0)
		for _, event := range invalidEvents {
			invalidIDs = append(invalidIDs, event.ID)
			if event.Type == models.RelayAccountEventTypeWithdraw {
				invalidWithdrawEventIDs = append(invalidWithdrawEventIDs, event.ID)
			}
			if event.Type == models.RelayAccountEventTypeDeposit {
				invalidDepositEventIDs = append(invalidDepositEventIDs, event.ID)
			}
		}
		if err := dbWithCtx.Model(&models.RelayAccountEvent{}).
			Where("id IN (?)", invalidIDs).
			Update("status", models.RelayAccountEventStatusInvalid).Error; err != nil {
			return err
		}
		if len(invalidWithdrawEventIDs) > 0 {
			if err := dbWithCtx.Model(&models.WithdrawRecord{}).
				Where("relay_account_event_id IN (?)", invalidWithdrawEventIDs).
				Where("local_status = ?", models.WithdrawLocalStatusPending).
				Update("local_status", models.WithdrawLocalStatusInvalid).Error; err != nil {
				return err
			}
		}
		if len(invalidDepositEventIDs) > 0 {
			if err := dbWithCtx.Model(&models.DepositRecord{}).
				Where("relay_account_event_id IN (?)", invalidDepositEventIDs).
				Where("local_status = ?", models.DepositLocalStatusPending).
				Update("local_status", models.DepositLocalStatusInvalid).Error; err != nil {
				return err
			}
		}
	}

	mergedAccounts := mergeRelayAccountEvents(validEvents)
	addresses := make([]string, 0, len(mergedAccounts))
	for address := range mergedAccounts {
		addresses = append(addresses, address)
	}
	eventIDs := make([]uint, 0, len(validEvents))
	for _, event := range validEvents {
		eventIDs = append(eventIDs, event.ID)
	}

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	appConfig := config.GetConfig()
	eventMACs := make(map[uint]string, len(validEvents))
	for _, event := range validEvents {
		eventMACs[event.ID] = utils.GenerateMAC([]byte(event.Reason), appConfig.MAC.SecretKey)
	}

	return db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var existedAccounts []models.RelayAccount
		if err := tx.Model(&models.RelayAccount{}).Where("address IN (?)", addresses).Find(&existedAccounts).Error; err != nil {
			return err
		}

		existedAddresses := make([]string, 0, len(existedAccounts))
		existedMap := make(map[string]*models.RelayAccount)
		for i, account := range existedAccounts {
			existedMap[account.Address] = &existedAccounts[i]
			existedAddresses = append(existedAddresses, account.Address)
		}

		newAccounts := make([]models.RelayAccount, 0)
		for address, amount := range mergedAccounts {
			if account, ok := existedMap[address]; ok {
				account.Balance.Int.Add(&account.Balance.Int, amount)
			} else {
				newAccounts = append(newAccounts, models.RelayAccount{
					Address: address,
					Balance: models.BigInt{Int: *new(big.Int).Set(amount)},
				})
			}
		}

		if len(newAccounts) > 0 {
			if err := tx.CreateInBatches(&newAccounts, 100).Error; err != nil {
				return err
			}
		}

		if len(existedMap) > 0 {
			var cases string
			for _, account := range existedMap {
				cases += fmt.Sprintf(" WHEN address = '%s' THEN '%s'", account.Address, account.Balance.String())
			}
			if err := tx.Model(&models.RelayAccount{}).
				Where("address IN (?)", existedAddresses).
				Update("balance", gorm.Expr("CASE"+cases+" END")).Error; err != nil {
				return err
			}
		}

		var macCases string
		for eventID, mac := range eventMACs {
			macCases += fmt.Sprintf(" WHEN id = %d THEN '%s'", eventID, mac)
		}
		if err := tx.Model(&models.RelayAccountEvent{}).
			Where("id IN (?)", eventIDs).
			Where("status = ?", models.RelayAccountEventStatusPending).
			Updates(map[string]interface{}{
				"mac":    gorm.Expr("CASE" + macCases + " END"),
				"status": models.RelayAccountEventStatusProcessed,
			}).Error; err != nil {
			return err
		}

		withdrawEventIDs := make([]uint, 0)
		for _, event := range validEvents {
			if event.Type == models.RelayAccountEventTypeWithdraw {
				withdrawEventIDs = append(withdrawEventIDs, event.ID)
			}
		}
		if len(withdrawEventIDs) > 0 {
			var records []models.WithdrawRecord
			if err := tx.Model(&models.WithdrawRecord{}).
				Where("relay_account_event_id IN (?)", withdrawEventIDs).
				Where("local_status = ?", models.WithdrawLocalStatusPending).
				Find(&records).Error; err != nil {
				return err
			}

			if len(records) > 0 {
				recordIDs := make([]uint, 0, len(records))
				var recordMACCases string
				for _, record := range records {
					recordIDs = append(recordIDs, record.ID)
					recordMAC := utils.GenerateMAC([]byte(record.MACString()), appConfig.MAC.SecretKey)
					recordMACCases += fmt.Sprintf(" WHEN id = %d THEN '%s'", record.ID, recordMAC)
				}

				if err := tx.Model(&models.WithdrawRecord{}).
					Where("id IN (?)", recordIDs).
					Where("local_status = ?", models.WithdrawLocalStatusPending).
					Updates(map[string]interface{}{
						"mac":          gorm.Expr("CASE" + recordMACCases + " END"),
						"local_status": models.WithdrawLocalStatusProcessed,
					}).Error; err != nil {
					return err
				}
			}
		}

		depositEventIDs := make([]uint, 0)
		for _, event := range validEvents {
			if event.Type == models.RelayAccountEventTypeDeposit {
				depositEventIDs = append(depositEventIDs, event.ID)
			}
		}
		if len(depositEventIDs) > 0 {
			if err := tx.Model(&models.DepositRecord{}).
				Where("relay_account_event_id IN (?)", depositEventIDs).
				Where("local_status = ?", models.DepositLocalStatusPending).
				Update("local_status", models.DepositLocalStatusProcessed).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func syncRelayAccountsToDB(ctx context.Context, db *gorm.DB) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			for {
				events, err := getPendingRelayAccountEvents(ctx, db, 50)
				if err != nil {
					log.Errorf("Failed to load pending relay account events: %v", err)
					break
				}
				if len(events) == 0 {
					break
				}
				if err := processPendingRelayAccountEvents(ctx, db, events); err != nil {
					log.Errorf("Failed to process pending relay account events: %v", err)
					break
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}
	}
}

func getRelayAccountFromCache(ctx context.Context, db *gorm.DB, address string) (*big.Int, error) {
	relayAccountCache.mu.RLock()
	balance, exists := relayAccountCache.accounts[address]
	relayAccountCache.mu.RUnlock()
	if exists {
		return balance, nil
	}

	relayAccountCache.mu.Lock()
	defer relayAccountCache.mu.Unlock()
	if balance, exists := relayAccountCache.accounts[address]; exists {
		return balance, nil
	}

	var dbAccount models.RelayAccount
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).
		Where("address = ?", address).
		Attrs(models.RelayAccount{Balance: models.BigInt{Int: *big.NewInt(0)}}).
		FirstOrInit(&dbAccount).Error; err != nil {
		return nil, err
	}
	relayAccountCache.accounts[address] = &dbAccount.Balance.Int
	return relayAccountCache.accounts[address], nil
}

func createRelayAccountEventWithID(ctx context.Context, db *gorm.DB, eventType models.RelayAccountEventType, reason, address string, amount *big.Int) (uint, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	event := &models.RelayAccountEvent{
		Reason:    reason,
		Address:   address,
		Amount:    models.BigInt{Int: *new(big.Int).Set(amount)},
		CreatedAt: time.Now(),
		Status:    models.RelayAccountEventStatusPending,
		Type:      eventType,
	}
	if err := db.WithContext(dbCtx).Create(event).Error; err != nil {
		return 0, err
	}
	return event.ID, nil
}

func createRelayAccountEvent(ctx context.Context, db *gorm.DB, eventType models.RelayAccountEventType, reason, address string, amount *big.Int) error {
	_, err := createRelayAccountEventWithID(ctx, db, eventType, reason, address, amount)
	return err
}

func createRelayAccountEvents(ctx context.Context, db *gorm.DB, events []models.RelayAccountEvent) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Create(&events).Error
}

func depositRelayAccount(ctx context.Context, db *gorm.DB, txHash, address string, amount *big.Int, network string) (func() error, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		reason := fmt.Sprintf("%d-%s-%s", models.RelayAccountEventTypeDeposit, txHash, network)
		eventID, err := createRelayAccountEventWithID(ctx, tx, models.RelayAccountEventTypeDeposit, reason, address, amount)
		if err != nil {
			return err
		}
		record := &models.DepositRecord{
			Address:             address,
			Amount:              models.BigInt{Int: *new(big.Int).Set(amount)},
			Network:             network,
			TxHash:              txHash,
			RelayAccountEventID: eventID,
			LocalStatus:         models.DepositLocalStatusPending,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	balance, err := getRelayAccountFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}
	amountCopy := new(big.Int).Set(amount)
	return func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		balance.Add(balance, amountCopy)
		return nil
	}, nil
}

func chargeTaskFromRelayAccount(ctx context.Context, db *gorm.DB, taskIDCommitment, address string, amount *big.Int) (func() error, error) {
	balance, err := getRelayAccountFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}
	if balance.Cmp(amount) < 0 {
		return nil, ErrInsufficientRelayAccount
	}
	reason := fmt.Sprintf("%d-%s", models.RelayAccountEventTypeTaskPayment, taskIDCommitment)
	if err := createRelayAccountEvent(ctx, db, models.RelayAccountEventTypeTaskPayment, reason, address, amount); err != nil {
		return nil, err
	}
	amountCopy := new(big.Int).Set(amount)
	return func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		if balance.Cmp(amountCopy) < 0 {
			return ErrInsufficientRelayAccount
		}
		balance.Sub(balance, amountCopy)
		return nil
	}, nil
}

func refundTaskPaymentToRelayAccount(ctx context.Context, db *gorm.DB, taskIDCommitment, address string, amount *big.Int) (func() error, error) {
	reason := fmt.Sprintf("%d-%s", models.RelayAccountEventTypeTaskRefund, taskIDCommitment)
	if err := createRelayAccountEvent(ctx, db, models.RelayAccountEventTypeTaskRefund, reason, address, amount); err != nil {
		return nil, err
	}
	balance, err := getRelayAccountFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}
	amountCopy := new(big.Int).Set(amount)
	return func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		balance.Add(balance, amountCopy)
		return nil
	}, nil
}

func sendTaskIncome(ctx context.Context, db *gorm.DB, taskIDCommitment, address string, amount *big.Int, taskType models.TaskType, network string) (func() error, error) {
	appConfig := config.GetConfig()
	daoTaskShare := big.NewInt(0).Mul(amount, big.NewInt(0).SetUint64(appConfig.Dao.TaskFeeSharePercent))
	daoTaskShare.Div(daoTaskShare, big.NewInt(100))
	nodeIncome := big.NewInt(0).Sub(amount, daoTaskShare)
	delegatorShare := GetDelegatorShare(address)

	rewardEvent := models.RelayAccountEvent{
		Address:   address,
		Amount:    models.BigInt{Int: *new(big.Int).Set(nodeIncome)},
		CreatedAt: time.Now(),
		Status:    models.RelayAccountEventStatusPending,
		Type:      models.RelayAccountEventTypeTaskIncome,
		Reason:    fmt.Sprintf("%d-%s", models.RelayAccountEventTypeTaskIncome, taskIDCommitment),
	}
	daoEvent := models.RelayAccountEvent{
		Address:   appConfig.Dao.TaskFeeShareAddress,
		Amount:    models.BigInt{Int: *new(big.Int).Set(daoTaskShare)},
		CreatedAt: time.Now(),
		Status:    models.RelayAccountEventStatusPending,
		Type:      models.RelayAccountEventTypeDaoTaskShare,
		Reason:    fmt.Sprintf("%d-%s", models.RelayAccountEventTypeDaoTaskShare, taskIDCommitment),
	}
	events := []models.RelayAccountEvent{rewardEvent, daoEvent}

	totalDelegatorFee := big.NewInt(0)
	if delegatorShare > 0 {
		userStakings := GetDelegationsOfNode(address, network)
		if len(userStakings) > 0 {
			totalDelegatorFee.Mul(nodeIncome, big.NewInt(int64(delegatorShare)))
			totalDelegatorFee.Div(totalDelegatorFee, big.NewInt(100))
			nodeIncome = nodeIncome.Sub(nodeIncome, totalDelegatorFee)

			totalUserStakeAmount := big.NewInt(0)
			for _, amount := range userStakings {
				totalUserStakeAmount = totalUserStakeAmount.Add(totalUserStakeAmount, amount)
			}
			userAddresses := make([]string, 0, len(userStakings))
			userDelegatorFees := make([]*big.Int, 0, len(userStakings))
			dispatchedDelegatorFee := big.NewInt(0)
			for userAddress, userStakingAmount := range userStakings {
				userAddresses = append(userAddresses, userAddress)
				delegatorFee := big.NewInt(0).Mul(totalDelegatorFee, userStakingAmount)
				delegatorFee = delegatorFee.Div(delegatorFee, totalUserStakeAmount)
				userDelegatorFees = append(userDelegatorFees, delegatorFee)
				dispatchedDelegatorFee = dispatchedDelegatorFee.Add(dispatchedDelegatorFee, delegatorFee)
			}
			userDelegatorFees[0].Add(userDelegatorFees[0], big.NewInt(0).Sub(totalDelegatorFee, dispatchedDelegatorFee))

			for i := range len(userStakings) {
				events = append(events, models.RelayAccountEvent{
					Address:   userAddresses[i],
					Amount:    models.BigInt{Int: *userDelegatorFees[i]},
					CreatedAt: time.Now(),
					Status:    models.RelayAccountEventStatusPending,
					Type:      models.RelayAccountEventTypeUserDelegation,
					Reason:    fmt.Sprintf("%d-%s-%s", models.RelayAccountEventTypeUserDelegation, taskIDCommitment, userAddresses[i]),
				})
			}
		}
	}
	incentive, _ := utils.WeiToEther(nodeIncome).Float64()

	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createRelayAccountEvents(ctx, tx, events); err != nil {
			return err
		}
		if err := addNodeEarning(ctx, tx, address, nodeIncome, totalDelegatorFee); err != nil {
			return err
		}
		for _, event := range events {
			if event.Type == models.RelayAccountEventTypeUserDelegation {
				if err := addUserStakingEarning(ctx, tx, event.Address, address, network, &event.Amount.Int); err != nil {
					return err
				}
				if err := addUserEarning(ctx, tx, event.Address, &event.Amount.Int); err != nil {
					return err
				}
			}
		}

		if err := addNodeIncentive(ctx, tx, address, incentive, taskType); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	balances := make([]*big.Int, 0, len(events))
	for _, event := range events {
		balance, err := getRelayAccountFromCache(ctx, db, event.Address)
		if err != nil {
			return nil, err
		}
		balances = append(balances, balance)
	}

	return func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		for i := range events {
			balances[i].Add(balances[i], &events[i].Amount.Int)
		}
		return nil
	}, nil
}

func chargeWithdrawFromRelayAccount(ctx context.Context, db *gorm.DB, withdrawID uint, address string, amount *big.Int) (uint, func() error, error) {
	balance, err := getRelayAccountFromCache(ctx, db, address)
	if err != nil {
		return 0, nil, err
	}
	if balance.Cmp(amount) < 0 {
		return 0, nil, ErrInsufficientRelayAccount
	}
	reason := fmt.Sprintf("%d-%d", models.RelayAccountEventTypeWithdraw, withdrawID)
	event := &models.RelayAccountEvent{
		Reason:    reason,
		Address:   address,
		Amount:    models.BigInt{Int: *new(big.Int).Set(amount)},
		CreatedAt: time.Now(),
		Status:    models.RelayAccountEventStatusPending,
		Type:      models.RelayAccountEventTypeWithdraw,
	}
	if err := db.Create(event).Error; err != nil {
		return 0, nil, err
	}

	amountCopy := new(big.Int).Set(amount)
	return event.ID, func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		if balance.Cmp(amountCopy) < 0 {
			return ErrInsufficientRelayAccount
		}
		balance.Sub(balance, amountCopy)
		return nil
	}, nil
}

func fulfillWithdrawFeeIncome(ctx context.Context, db *gorm.DB, withdrawID uint, feeAddress string, fee *big.Int) (func() error, error) {
	reason := fmt.Sprintf("%d-%d", models.RelayAccountEventTypeWithdrawFeeIncome, withdrawID)
	if err := createRelayAccountEvent(ctx, db, models.RelayAccountEventTypeWithdrawFeeIncome, reason, feeAddress, fee); err != nil {
		return nil, err
	}
	balance, err := getRelayAccountFromCache(ctx, db, feeAddress)
	if err != nil {
		return nil, err
	}
	feeCopy := new(big.Int).Set(fee)
	return func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		balance.Add(balance, feeCopy)
		return nil
	}, nil
}

func rejectWithdrawToRelayAccount(ctx context.Context, db *gorm.DB, withdrawID uint, address string, amount *big.Int) (func() error, error) {
	reason := fmt.Sprintf("%d-%d", models.RelayAccountEventTypeWithdrawRefund, withdrawID)
	if err := createRelayAccountEvent(ctx, db, models.RelayAccountEventTypeWithdrawRefund, reason, address, amount); err != nil {
		return nil, err
	}
	balance, err := getRelayAccountFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}
	amountCopy := new(big.Int).Set(amount)
	return func() error {
		relayAccountCache.mu.Lock()
		defer relayAccountCache.mu.Unlock()
		balance.Add(balance, amountCopy)
		return nil
	}, nil
}

func GetRelayAccountBalance(ctx context.Context, db *gorm.DB, address string) (*big.Int, error) {
	return getRelayAccountFromCache(ctx, db, address)
}
