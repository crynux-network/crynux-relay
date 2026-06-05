package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"errors"
	"math/big"
	"time"

	"gorm.io/gorm"
)

var ErrWithdrawRequestNotPending = errors.New("withdraw request is not pending")
var ErrWithdrawRequestNotProcessedLocally = errors.New("withdraw request has not been processed locally")

func Withdraw(ctx context.Context, db *gorm.DB, address, benefitAddress string, amount *big.Int, network string) (*models.WithdrawRecord, error) {
	appConfig := config.GetConfig()
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	networkConfig, ok := appConfig.GetEffectiveFundingNetwork(network)
	if !ok {
		return nil, errors.New("unsupported withdraw network")
	}
	withdrawalFee := utils.EtherToWei(big.NewInt(0).SetUint64(networkConfig.WithdrawalFee))
	if address == appConfig.Withdraw.WithdrawalFeeAddress || address == appConfig.Dao.TaskFeeShareAddress {
		withdrawalFee = big.NewInt(0)
	}
	record := &models.WithdrawRecord{
		Address:        address,
		BenefitAddress: benefitAddress,
		Amount:         models.BigInt{Int: *amount},
		Network:        network,
		Status:         models.WithdrawStatusPending,
		LocalStatus:    models.WithdrawLocalStatusPending,
		WithdrawalFee:  models.BigInt{Int: *withdrawalFee},
	}

	totalAmount := big.NewInt(0).Add(amount, withdrawalFee)

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		eventID, commitFunc, err := chargeWithdrawFromRelayAccount(ctx, tx, record.ID, address, totalAmount)
		if err != nil {
			return err
		}
		if err := tx.Model(&models.WithdrawRecord{}).Where("id = ?", record.ID).Update("relay_account_event_id", eventID).Error; err != nil {
			return err
		}
		record.RelayAccountEventID = eventID
		if err := commitFunc(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return record, nil
}

func FulfillWithdraw(ctx context.Context, db *gorm.DB, withdrawID uint, txHash string) error {
	appConfig := config.GetConfig()

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var record models.WithdrawRecord
		if err := tx.Model(&models.WithdrawRecord{}).Where("id = ?", withdrawID).First(&record).Error; err != nil {
			return err
		}

		if record.Status == models.WithdrawStatusSuccess {
			return nil
		}

		if record.LocalStatus != models.WithdrawLocalStatusProcessed {
			return ErrWithdrawRequestNotProcessedLocally
		}

		if record.Status != models.WithdrawStatusPending {
			return ErrWithdrawRequestNotPending
		}

		updates := map[string]interface{}{
			"status":  models.WithdrawStatusSuccess,
			"tx_hash": txHash,
		}
		if err := tx.Model(&models.WithdrawRecord{}).Where("id = ?", withdrawID).Updates(updates).Error; err != nil {
			return err
		}

		if record.WithdrawalFee.Cmp(big.NewInt(0)) > 0 {
			commitFunc, err := fulfillWithdrawFeeIncome(ctx, tx, record.ID, appConfig.Withdraw.WithdrawalFeeAddress, &record.WithdrawalFee.Int)
			if err != nil {
				return err
			}
			if err := commitFunc(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func RejectWithdraw(ctx context.Context, db *gorm.DB, withdrawID uint) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var record models.WithdrawRecord
		if err := tx.Model(&models.WithdrawRecord{}).Where("id = ?", withdrawID).First(&record).Error; err != nil {
			return err
		}

		if record.Status == models.WithdrawStatusFailed {
			return nil
		}

		if record.LocalStatus != models.WithdrawLocalStatusProcessed {
			return ErrWithdrawRequestNotProcessedLocally
		}

		if record.Status != models.WithdrawStatusPending {
			return ErrWithdrawRequestNotPending
		}

		if err := tx.Model(&models.WithdrawRecord{}).Where("id = ?", withdrawID).Update("status", models.WithdrawStatusFailed).Error; err != nil {
			return err
		}

		totalAmount := big.NewInt(0).Add(&record.Amount.Int, &record.WithdrawalFee.Int)

		commitFunc, err := rejectWithdrawToRelayAccount(ctx, tx, record.ID, record.Address, totalAmount)
		if err != nil {
			return err
		}
		if err := commitFunc(); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}
