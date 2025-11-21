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

var ErrInsufficientTaskFee = errors.New("insufficient task fee")

type TaskFeeCache struct {
	taskFees map[string]*big.Int
	mu       sync.RWMutex
}

var taskFeeCache = &TaskFeeCache{
	taskFees: make(map[string]*big.Int),
}

func InitTaskFeeCache(ctx context.Context, db *gorm.DB) error {
	for {
		events, err := getPendingTaskFeeEvents(ctx, db, 50)
		if err != nil {
			return err
		}

		if len(events) == 0 {
			break
		}

		if err := processPendingTaskFeeEvents(ctx, db, events); err != nil {
			return err
		}
	}
	for {
		events, err := getPendingWithdrawEvents(ctx, db, 50)
		if err != nil {
			return err
		}

		if len(events) == 0 {
			break
		}

		if err := processPendingWithdrawEvents(ctx, db, events); err != nil {
			return err
		}
	}
	return nil
}

func getPendingTaskFeeEvents(ctx context.Context, db *gorm.DB, limit int) ([]models.TaskFeeEvent, error) {
	var events []models.TaskFeeEvent
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := db.WithContext(dbCtx).Where("status = ?", models.TaskFeeEventStatusPending).Order("id").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, err
	}

	return events, nil
}

func getPendingWithdrawEvents(ctx context.Context, db *gorm.DB, limit int) ([]models.WithdrawRecord, error) {
	var withdrawRecords []models.WithdrawRecord
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := db.WithContext(dbCtx).Where("local_status = ?", models.WithdrawLocalStatusPending).Order("id").Limit(limit).Find(&withdrawRecords).Error
	if err != nil {
		return nil, err
	}

	return withdrawRecords, nil
}

func validatePendingTaskFeeEvents(ctx context.Context, db *gorm.DB, events []models.TaskFeeEvent) ([]models.TaskFeeEvent, []models.TaskFeeEvent, error) {
	invalidEvents := make([]models.TaskFeeEvent, 0)
	candidateEvents := make([]models.TaskFeeEvent, 0)

	var taskIDCommitments []string
	taskIDCommitmentMap := make(map[string]struct{})
	var withdrawIDs []uint
	var txHashes []string
	var networks []string
	for _, event := range events {
		reasons := strings.Split(event.Reason, "-")
		if len(reasons) != 2 && len(reasons) != 3 {
			invalidEvents = append(invalidEvents, event)
			continue
		}
		eventType := reasons[0]
		if eventType != fmt.Sprintf("%d", event.Type) {
			invalidEvents = append(invalidEvents, event)
			continue
		}
		if event.Type == models.TaskFeeEventTypeTask || event.Type == models.TaskFeeEventTypeDraw || event.Type == models.TaskFeeEventTypeUserCommission {
			taskIDCommitment := reasons[1]
			if _, ok := taskIDCommitmentMap[taskIDCommitment]; !ok {
				taskIDCommitmentMap[taskIDCommitment] = struct{}{}
				taskIDCommitments = append(taskIDCommitments, taskIDCommitment)
			}
		} else if event.Type == models.TaskFeeEventTypeWithdrawalFee {
			withdrawID, err := strconv.ParseUint(reasons[1], 10, 64)
			if err != nil {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			withdrawIDs = append(withdrawIDs, uint(withdrawID))
		} else if event.Type == models.TaskFeeEventTypeBought {
			if len(reasons) != 3 {
				invalidEvents = append(invalidEvents, event)
				continue
			}
			txHash := reasons[1]
			network := reasons[2]
			txHashes = append(txHashes, txHash)
			networks = append(networks, network)
		}
		candidateEvents = append(candidateEvents, event)
	}

	var tasks []models.InferenceTask
	if err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.WithContext(dbCtx).
			Where("task_id_commitment IN (?)", taskIDCommitments).
			Find(&tasks).Error; err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return nil, nil, err
	}
	taskMap := make(map[string]models.InferenceTask)
	for _, task := range tasks {
		taskMap[task.TaskIDCommitment] = task
	}

	var withdrawRecords []models.WithdrawRecord
	if err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.WithContext(dbCtx).Where("id IN (?)", withdrawIDs).Find(&withdrawRecords).Error; err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return nil, nil, err
	}
	withdrawRecordMap := make(map[uint]models.WithdrawRecord)
	for _, withdrawRecord := range withdrawRecords {
		withdrawRecordMap[withdrawRecord.ID] = withdrawRecord
	}

	validTxHashesMap := make(map[string]struct{})
	for i, txHash := range txHashes {
		network := networks[i]
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
		validTxHashesMap[txHash] = struct{}{}
	}

	validEvents := make([]models.TaskFeeEvent, 0)
	for _, event := range candidateEvents {
		reasons := strings.Split(event.Reason, "-")
		if event.Type == models.TaskFeeEventTypeTask || event.Type == models.TaskFeeEventTypeDraw || event.Type == models.TaskFeeEventTypeUserCommission {
			taskIDCommitment := reasons[1]
			if task, exists := taskMap[taskIDCommitment]; exists {
				if task.Status == models.TaskValidated || task.Status == models.TaskGroupValidated {
					continue
				} else if task.Status == models.TaskEndSuccess || task.Status == models.TaskEndGroupSuccess || task.Status == models.TaskEndGroupRefund {
					if event.TaskFee.Int.Cmp(&task.TaskFee.Int) > 0 {
						invalidEvents = append(invalidEvents, event)
						continue
					}
					validEvents = append(validEvents, event)
				} else {
					invalidEvents = append(invalidEvents, event)
				}
			} else {
				invalidEvents = append(invalidEvents, event)
			}
		} else if event.Type == models.TaskFeeEventTypeWithdrawalFee {
			withdrawID, _ := strconv.ParseUint(reasons[1], 10, 64)
			if _, exists := withdrawRecordMap[uint(withdrawID)]; exists {
				validEvents = append(validEvents, event)
			} else {
				invalidEvents = append(invalidEvents, event)
			}
		} else if event.Type == models.TaskFeeEventTypeBought {
			txHash := reasons[1]
			if _, exists := validTxHashesMap[txHash]; exists {
				validEvents = append(validEvents, event)
			} else {
				invalidEvents = append(invalidEvents, event)
			}
		} else {
			invalidEvents = append(invalidEvents, event)
		}
	}

	return validEvents, invalidEvents, nil
}

func validatePendingWithdrawEvents(ctx context.Context, db *gorm.DB, events []models.WithdrawRecord) ([]models.WithdrawRecord, []models.WithdrawRecord, error) {
	var addresses []string
	for _, event := range events {
		addresses = append(addresses, event.Address)
	}

	taskFees, err := models.GetTaskFees(ctx, db, addresses)
	if err != nil {
		return nil, nil, err
	}

	lastProcessedTaskFeeEventID, err := models.GetLastProcessedTaskFeeEventID(ctx, db)
	if err != nil {
		return nil, nil, err
	}

	existedTaskFeeAddressMap := make(map[string]*models.TaskFee)
	for _, taskFee := range taskFees {
		existedTaskFeeAddressMap[taskFee.Address] = &taskFee
	}

	validEvents := make([]models.WithdrawRecord, 0)
	invalidEvents := make([]models.WithdrawRecord, 0)
	for _, event := range events {
		if event.TaskFeeEventID > lastProcessedTaskFeeEventID {
			continue
		}
		if taskFee, exists := existedTaskFeeAddressMap[event.Address]; exists {
			if event.Status != models.WithdrawStatusFailed && taskFee.TaskFee.Int.Cmp(&event.Amount.Int) < 0 {
				invalidEvents = append(invalidEvents, event)
			} else {
				validEvents = append(validEvents, event)
			}
		} else {
			invalidEvents = append(invalidEvents, event)
		}
	}
	return validEvents, invalidEvents, nil
}

func getTaskFeeFromCache(ctx context.Context, db *gorm.DB, address string) (*big.Int, error) {
	taskFeeCache.mu.RLock()
	taskFee, exists := taskFeeCache.taskFees[address]
	taskFeeCache.mu.RUnlock()

	if exists {
		return taskFee, nil
	}

	taskFeeCache.mu.Lock()
	defer taskFeeCache.mu.Unlock()

	if taskFee, exists := taskFeeCache.taskFees[address]; exists {
		return taskFee, nil
	}

	var dbTaskFee models.TaskFee
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.WithContext(dbCtx).Where("address = ?", address).Attrs(models.TaskFee{TaskFee: models.BigInt{Int: *big.NewInt(0)}}).FirstOrInit(&dbTaskFee).Error; err != nil {
		return nil, err
	}

	taskFeeCache.taskFees[address] = &dbTaskFee.TaskFee.Int

	return taskFeeCache.taskFees[address], nil
}

func StartTaskFeeSync(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syncTaskFeesToDB(ctx, db); err != nil {
				log.Errorf("Failed to sync task fees: %v", err)
			}
		}
	}
}

func mergeTaskFeeEvents(events []models.TaskFeeEvent) map[string]*big.Int {
	mergedTaskFees := make(map[string]*big.Int)
	for _, event := range events {
		if _, exists := mergedTaskFees[event.Address]; !exists {
			mergedTaskFees[event.Address] = big.NewInt(0).Set(&event.TaskFee.Int)
		} else {
			mergedTaskFees[event.Address].Add(mergedTaskFees[event.Address], &event.TaskFee.Int)
		}
	}
	return mergedTaskFees
}

func mergeWithdrawEvents(events []models.WithdrawRecord) map[string]*big.Int {
	mergedWithdrawFees := make(map[string]*big.Int)
	for _, event := range events {
		if _, exists := mergedWithdrawFees[event.Address]; !exists {
			mergedWithdrawFees[event.Address] = big.NewInt(0)
		}
		if event.Status == models.WithdrawStatusFailed {
			mergedWithdrawFees[event.Address].Add(mergedWithdrawFees[event.Address], &event.Amount.Int)
		} else {
			mergedWithdrawFees[event.Address].Sub(mergedWithdrawFees[event.Address], &event.Amount.Int)
		}
	}
	return mergedWithdrawFees
}

func processPendingTaskFeeEvents(ctx context.Context, db *gorm.DB, events []models.TaskFeeEvent) error {
	validEvents, invalidEvents, err := validatePendingTaskFeeEvents(ctx, db, events)
	if err != nil {
		return err
	}

	if len(invalidEvents) > 0 {
		var invalidEventIDs []uint
		for _, event := range invalidEvents {
			invalidEventIDs = append(invalidEventIDs, event.ID)
		}
		if err := db.Model(&models.TaskFeeEvent{}).Where("id IN (?)", invalidEventIDs).Update("status", models.TaskFeeEventStatusInvalid).Error; err != nil {
			return err
		}
	}
	mergedTaskFees := mergeTaskFeeEvents(validEvents)

	var eventIDs []uint
	for _, event := range validEvents {
		eventIDs = append(eventIDs, event.ID)
	}

	var addresses []string
	for address := range mergedTaskFees {
		addresses = append(addresses, address)
	}

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	appConfig := config.GetConfig()
	eventMACs := make(map[uint]string)
	for _, event := range validEvents {
		eventMACs[event.ID] = utils.GenerateMAC([]byte(event.Reason), appConfig.MAC.SecretKey)
	}

	return db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var existedTaskFees []models.TaskFee
		if err := tx.Model(&models.TaskFee{}).Where("address IN (?)", addresses).Find(&existedTaskFees).Error; err != nil {
			return err
		}

		existedAddresses := make([]string, len(existedTaskFees))
		existedTaskFeesMap := make(map[string]*models.TaskFee)
		for i, taskFee := range existedTaskFees {
			existedTaskFeesMap[taskFee.Address] = &existedTaskFees[i]
			existedAddresses[i] = taskFee.Address
		}

		var newTaskFees []models.TaskFee
		for address, amount := range mergedTaskFees {
			if taskFee, exists := existedTaskFeesMap[address]; !exists {
				newTaskFees = append(newTaskFees, models.TaskFee{Address: address, TaskFee: models.BigInt{Int: *amount}})
			} else {
				taskFee.TaskFee.Int.Add(&taskFee.TaskFee.Int, amount)
			}
		}

		if len(newTaskFees) > 0 {
			if err := tx.CreateInBatches(&newTaskFees, 100).Error; err != nil {
				return err
			}
		}

		if len(existedTaskFeesMap) > 0 {
			var cases string
			for _, taskFee := range existedTaskFeesMap {
				cases += fmt.Sprintf(" WHEN address = '%s' THEN '%s'", taskFee.Address, taskFee.TaskFee.String())
			}
			if err := tx.Model(&models.TaskFee{}).Where("address IN (?)", existedAddresses).
				Update("task_fee", gorm.Expr("CASE"+cases+" END")).Error; err != nil {
				return err
			}
		}

		var macCases string
		for id, mac := range eventMACs {
			macCases += fmt.Sprintf(" WHEN id = '%d' THEN '%s'", id, mac)
		}
		updates := map[string]interface{}{
			"mac":    gorm.Expr("CASE" + macCases + " END"),
			"status": models.TaskFeeEventStatusProcessed,
		}

		if err := tx.Model(&models.TaskFeeEvent{}).Where("id IN (?)", eventIDs).Where("status = ?", models.TaskFeeEventStatusPending).Updates(updates).Error; err != nil {
			return err
		}

		return nil
	})
}

func processPendingWithdrawEvents(ctx context.Context, db *gorm.DB, events []models.WithdrawRecord) error {
	validEvents, invalidEvents, err := validatePendingWithdrawEvents(ctx, db, events)
	if err != nil {
		return err
	}

	if len(invalidEvents) != 0 {
		var invalidEventIDs []uint
		for _, event := range invalidEvents {
			invalidEventIDs = append(invalidEventIDs, event.ID)
		}
		if err := db.Model(&models.WithdrawRecord{}).Where("id IN (?)", invalidEventIDs).Update("local_status", models.WithdrawLocalStatusInvalid).Error; err != nil {
			return err
		}
	}

	mergedWithdrawFees := mergeWithdrawEvents(validEvents)

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var eventIDs []uint
	for _, event := range validEvents {
		eventIDs = append(eventIDs, event.ID)
	}

	var addresses []string
	for address := range mergedWithdrawFees {
		addresses = append(addresses, address)
	}

	appConfig := config.GetConfig()
	eventMACs := make(map[uint]string)
	for _, event := range validEvents {
		eventMACs[event.ID] = utils.GenerateMAC([]byte(event.MACString()), appConfig.MAC.SecretKey)
	}

	return db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var existedTaskFees []models.TaskFee
		if err := tx.Model(&models.TaskFee{}).Where("address IN (?)", addresses).Find(&existedTaskFees).Error; err != nil {
			return err
		}

		existedAddresses := make([]string, len(existedTaskFees))
		existedTaskFeesMap := make(map[string]*models.TaskFee)
		for i, taskFee := range existedTaskFees {
			existedTaskFeesMap[taskFee.Address] = &existedTaskFees[i]
			existedAddresses[i] = taskFee.Address
		}

		for address, amount := range mergedWithdrawFees {
			if taskFee, exists := existedTaskFeesMap[address]; exists {
				taskFee.TaskFee.Int.Add(&taskFee.TaskFee.Int, amount)
			}
		}

		var cases string
		for _, taskFee := range existedTaskFeesMap {
			cases += fmt.Sprintf(" WHEN address = '%s' THEN '%s'", taskFee.Address, taskFee.TaskFee.String())
		}
		if err := tx.Model(&models.TaskFee{}).Where("address IN (?)", existedAddresses).
			Update("task_fee", gorm.Expr("CASE"+cases+" END")).Error; err != nil {
			return err
		}

		var macCases string
		for id, mac := range eventMACs {
			macCases += fmt.Sprintf(" WHEN id = '%d' THEN '%s'", id, mac)
		}
		updates := map[string]interface{}{
			"mac":          gorm.Expr("CASE" + macCases + " END"),
			"local_status": models.WithdrawLocalStatusProcessed,
		}

		if err := tx.Model(&models.WithdrawRecord{}).Where("id IN (?)", eventIDs).Where("local_status = ?", models.WithdrawLocalStatusPending).Updates(updates).Error; err != nil {
			return err
		}

		return nil
	})
}

func syncTaskFeesToDB(ctx context.Context, db *gorm.DB) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := func() error {
				for {
					events, err := getPendingTaskFeeEvents(ctx, db, 50)
					if err != nil {
						return err
					}

					if len(events) == 0 {
						break
					}

					if err := processPendingTaskFeeEvents(ctx, db, events); err != nil {
						return err
					}
				}
				return nil
			}()

			if err != nil {
				log.Errorf("Failed to sync task fee events: %v", err)
			}

			err = func() error {
				for {
					events, err := getPendingWithdrawEvents(ctx, db, 50)
					if err != nil {
						return err
					}

					if len(events) == 0 {
						break
					}

					if err := processPendingWithdrawEvents(ctx, db, events); err != nil {
						return err
					}
				}
				return nil
			}()

			if err != nil {
				log.Errorf("Failed to sync withdraw events: %v", err)
			}

			// Wait for 2 seconds before next iteration
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}
	}
}

func buyTaskFee(ctx context.Context, db *gorm.DB, txHash, address string, amount *big.Int, network string) (func() error, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		event := &models.TaskFeeEvent{
			Address:   address,
			TaskFee:   models.BigInt{Int: *new(big.Int).Set(amount)},
			CreatedAt: time.Now(),
			Status:    models.TaskFeeEventStatusPending,
			Type:      models.TaskFeeEventTypeBought,
			Reason:    fmt.Sprintf("%d-%s-%s", models.TaskFeeEventTypeBought, txHash, network),
		}
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		depositRecord := &models.DepositRecord{
			Address: address,
			Amount:  models.BigInt{Int: *new(big.Int).Set(amount)},
			Network: network,
			TxHash:  txHash,
		}
		if err := tx.Create(depositRecord).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	taskFee, err := getTaskFeeFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}
	amountCopy := new(big.Int).Set(amount)
	commitFunc := func() error {
		taskFeeCache.mu.Lock()
		defer taskFeeCache.mu.Unlock()
		taskFee.Add(taskFee, amountCopy)
		return nil
	}

	return commitFunc, nil
}

func sendTaskFee(ctx context.Context, db *gorm.DB, taskIDCommitment, address string, amount *big.Int, taskType models.TaskType, network string) (func() error, error) {
	appConfig := config.GetConfig()
	daoFee := big.NewInt(0).Mul(amount, big.NewInt(0).SetUint64(appConfig.Dao.Percent))
	daoFee.Div(daoFee, big.NewInt(100))

	reward := big.NewInt(0).Sub(amount, daoFee)
	totalDelegatorFee := big.NewInt(0)
	delegatorShare := GetDelegatorShare(address)
	if delegatorShare > 0 {
		totalCommissionFee := totalDelegatorFee.Mul(reward, big.NewInt(int64(delegatorShare)))
		totalCommissionFee.Div(totalCommissionFee, big.NewInt(100))
		reward = reward.Sub(reward, totalCommissionFee)
	}

	rewardEvent := &models.TaskFeeEvent{
		Address:   address,
		TaskFee:   models.BigInt{Int: *new(big.Int).Set(reward)},
		CreatedAt: time.Now(),
		Status:    models.TaskFeeEventStatusPending,
		Type:      models.TaskFeeEventTypeTask,
		Reason:    fmt.Sprintf("%d-%s", models.TaskFeeEventTypeTask, taskIDCommitment),
	}
	daoEvent := &models.TaskFeeEvent{
		Address:   appConfig.Dao.Address,
		TaskFee:   models.BigInt{Int: *new(big.Int).Set(daoFee)},
		CreatedAt: time.Now(),
		Status:    models.TaskFeeEventStatusPending,
		Type:      models.TaskFeeEventTypeDraw,
		Reason:    fmt.Sprintf("%d-%s", models.TaskFeeEventTypeDraw, taskIDCommitment),
	}
	events := []*models.TaskFeeEvent{rewardEvent, daoEvent}

	if totalDelegatorFee.Sign() > 0 {
		userStakings := GetDelegationsOfNode(address, network)
		totalUserStakeAmount := big.NewInt(0)
		for _, amount := range userStakings {
			totalUserStakeAmount = totalUserStakeAmount.Add(totalUserStakeAmount, amount)
		}
		userAddresses := make([]string, 0, len(userStakings))
		userDelegatorFees := make([]*big.Int, 0, len(userStakings))
		dispatchedCommissionFee := big.NewInt(0)
		for userAddress, userStakingAmount := range userStakings {
			userAddresses = append(userAddresses, userAddress)
			delegatorFee := big.NewInt(0).Mul(totalDelegatorFee, userStakingAmount)
			delegatorFee = delegatorFee.Div(delegatorFee, totalUserStakeAmount)
			userDelegatorFees = append(userDelegatorFees, delegatorFee)
			dispatchedCommissionFee = dispatchedCommissionFee.Add(dispatchedCommissionFee, delegatorFee)
		}
		userDelegatorFees[0].Add(userDelegatorFees[0], big.NewInt(0).Sub(totalDelegatorFee, dispatchedCommissionFee))

		for i := range len(userStakings) {
			events = append(events, &models.TaskFeeEvent{
				Address:   userAddresses[i],
				TaskFee:   models.BigInt{Int: *userDelegatorFees[i]},
				CreatedAt: time.Now(),
				Status:    models.TaskFeeEventStatusPending,
				Type:      models.TaskFeeEventTypeUserCommission,
				Reason:    fmt.Sprintf("%d-%s", models.TaskFeeEventTypeUserCommission, taskIDCommitment),
			})
			if err := addUserStakingEarning(ctx, db, userAddresses[i], address, network, userDelegatorFees[i]); err != nil {
				return nil, err
			}
			if err := addUserEarning(ctx, db, userAddresses[i], userDelegatorFees[i]); err != nil {
				return nil, err
			}
		}
	}

	if err := db.Create(events).Error; err != nil {
		return nil, err
	}

	if err := addNodeEarning(ctx, db, address, reward, totalDelegatorFee); err != nil {
		return nil, err
	}

	incentive, _ := utils.WeiToEther(reward).Float64()
	if err := addNodeIncentive(ctx, db, address, incentive, taskType); err != nil {
		return nil, err
	}

	taskFees := make([]*big.Int, 0, len(events))
	for _, event := range events {
		fee, err := getTaskFeeFromCache(ctx, db, event.Address)
		if err != nil {
			return nil, err
		}
		taskFees = append(taskFees, fee)
	}
	commitFunc := func() error {
		taskFeeCache.mu.Lock()
		defer taskFeeCache.mu.Unlock()
		for i := range len(events) {
			taskFees[i].Add(taskFees[i], &events[i].TaskFee.Int)
		}
		return nil
	}

	return commitFunc, nil
}

func withdrawTaskFee(ctx context.Context, db *gorm.DB, address string, amount *big.Int) (func() error, error) {
	taskFee, err := getTaskFeeFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}
	if taskFee.Cmp(amount) < 0 {
		return nil, ErrInsufficientTaskFee
	}
	amountCopy := new(big.Int).Set(amount)
	commitFunc := func() error {
		taskFeeCache.mu.Lock()
		defer taskFeeCache.mu.Unlock()
		if taskFee.Cmp(amountCopy) < 0 {
			return ErrInsufficientTaskFee
		}
		taskFee.Sub(taskFee, amountCopy)
		return nil
	}

	return commitFunc, nil
}

func fulfillWithdrawTaskFee(ctx context.Context, db *gorm.DB, withdrawID uint, withdrawalFeeAddress string, withdrawalFee *big.Int) (func() error, error) {
	event := &models.TaskFeeEvent{
		Address:   withdrawalFeeAddress,
		TaskFee:   models.BigInt{Int: *new(big.Int).Set(withdrawalFee)},
		CreatedAt: time.Now(),
		Status:    models.TaskFeeEventStatusPending,
		Type:      models.TaskFeeEventTypeWithdrawalFee,
		Reason:    fmt.Sprintf("%d-%d", models.TaskFeeEventTypeWithdrawalFee, withdrawID),
	}
	if err := db.Create(event).Error; err != nil {
		return nil, err
	}

	taskFee, err := getTaskFeeFromCache(ctx, db, withdrawalFeeAddress)
	if err != nil {
		return nil, err
	}

	amountCopy := new(big.Int).Set(withdrawalFee)
	commitFunc := func() error {
		taskFeeCache.mu.Lock()
		defer taskFeeCache.mu.Unlock()
		taskFee.Add(taskFee, amountCopy)
		return nil
	}

	return commitFunc, nil
}

func rejectWithdrawTaskFee(ctx context.Context, db *gorm.DB, address string, amount *big.Int) (func() error, error) {
	taskFee, err := getTaskFeeFromCache(ctx, db, address)
	if err != nil {
		return nil, err
	}

	amountCopy := new(big.Int).Set(amount)
	commitFunc := func() error {
		taskFeeCache.mu.Lock()
		defer taskFeeCache.mu.Unlock()
		taskFee.Add(taskFee, amountCopy)
		return nil
	}

	return commitFunc, nil
}

func GetTaskFee(ctx context.Context, db *gorm.DB, address string) (*big.Int, error) {
	return getTaskFeeFromCache(ctx, db, address)
}
