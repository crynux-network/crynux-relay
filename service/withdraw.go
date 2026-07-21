package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"errors"
	"math/big"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var ErrWithdrawRequestNotPending = errors.New("withdraw request is not pending")
var ErrWithdrawRequestNotProcessedLocally = errors.New("withdraw request has not been processed locally")
var ErrWithdrawDailyLimitExceeded = errors.New("daily withdrawal limit exceeded")

// CalculateWithdrawalFee returns the withdrawal fee in wei for the given withdraw
// amount: the network fixed fee plus a proportional fee. The proportional ratio is
// taken from the highest configured tier whose min_amount is not greater than the
// withdraw amount and is applied to the whole amount.
func CalculateWithdrawalFee(networkConfig config.EffectiveFundingNetworkConfig, amount *big.Int) *big.Int {
	fee := utils.EtherToWei(big.NewInt(0).SetUint64(networkConfig.WithdrawalFee))
	ratio := float64(0)
	for _, tier := range networkConfig.WithdrawalFeeTiers {
		tierMinAmount := utils.EtherToWei(big.NewInt(0).SetUint64(tier.MinAmount))
		if amount.Cmp(tierMinAmount) < 0 {
			break
		}
		ratio = tier.FeeRatio
	}
	if ratio > 0 {
		ratioRat := new(big.Rat).SetFloat64(ratio)
		proportionalFee := new(big.Int).Mul(amount, ratioRat.Num())
		proportionalFee.Quo(proportionalFee, ratioRat.Denom())
		fee.Add(fee, proportionalFee)
	}
	return fee
}

func Withdraw(ctx context.Context, db *gorm.DB, address, benefitAddress string, amount *big.Int, network string) (*models.WithdrawRecord, error) {
	appConfig := config.GetConfig()
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	networkConfig, ok := appConfig.GetEffectiveFundingNetwork(network)
	if !ok {
		return nil, errors.New("unsupported withdraw network")
	}
	withdrawalFee := CalculateWithdrawalFee(networkConfig, amount)
	if address == appConfig.Withdraw.WithdrawalFeeAddress || address == appConfig.Dao.TaskFeeShareAddress {
		withdrawalFee = big.NewInt(0)
	}
	log.Infof("Creating withdraw request, address: %s, benefit address: %s, network: %s, amount: %s, withdrawal fee: %s",
		address, benefitAddress, network, amount.String(), withdrawalFee.String())
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
		now := time.Now().UTC()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		var todayCount int64
		if err := tx.Model(&models.WithdrawRecord{}).
			Where("address = ? AND created_at >= ? AND status != ?", address, dayStart, models.WithdrawStatusFailed).
			Count(&todayCount).Error; err != nil {
			return err
		}
		if todayCount >= int64(appConfig.Withdraw.MaxWithdrawalsPerDay) {
			return ErrWithdrawDailyLimitExceeded
		}
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
		log.Infof("Created withdraw relay account event, withdraw id: %d, event id: %d, address: %s, network: %s, amount: %s, withdrawal fee: %s, total charged: %s",
			record.ID, eventID, address, network, amount.String(), withdrawalFee.String(), totalAmount.String())
		return nil
	}); err != nil {
		log.Infof("Failed to create withdraw request, address: %s, benefit address: %s, network: %s, amount: %s, withdrawal fee: %s, error: %v",
			address, benefitAddress, network, amount.String(), withdrawalFee.String(), err)
		return nil, err
	}

	log.Infof("Withdraw request created, withdraw id: %d, event id: %d, address: %s, benefit address: %s, network: %s, amount: %s, withdrawal fee: %s",
		record.ID, record.RelayAccountEventID, record.Address, record.BenefitAddress, record.Network, record.Amount.String(), record.WithdrawalFee.String())
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
		log.Infof("Fulfilling withdraw request, withdraw id: %d, address: %s, network: %s, amount: %s, withdrawal fee: %s, local status: %d, status: %d, tx hash: %s",
			record.ID, record.Address, record.Network, record.Amount.String(), record.WithdrawalFee.String(), record.LocalStatus, record.Status, txHash)

		if record.Status == models.WithdrawStatusSuccess {
			log.Infof("Withdraw request already fulfilled, withdraw id: %d, tx hash: %s", record.ID, record.TxHash.String)
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
			log.Infof("Created withdraw fee income event, withdraw id: %d, fee address: %s, withdrawal fee: %s",
				record.ID, appConfig.Withdraw.WithdrawalFeeAddress, record.WithdrawalFee.String())
		}
		log.Infof("Withdraw request fulfilled, withdraw id: %d, tx hash: %s", record.ID, txHash)
		return nil
	}); err != nil {
		log.Infof("Failed to fulfill withdraw request, withdraw id: %d, tx hash: %s, error: %v", withdrawID, txHash, err)
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
		log.Infof("Rejecting withdraw request, withdraw id: %d, address: %s, network: %s, amount: %s, withdrawal fee: %s, local status: %d, status: %d",
			record.ID, record.Address, record.Network, record.Amount.String(), record.WithdrawalFee.String(), record.LocalStatus, record.Status)

		if record.Status == models.WithdrawStatusFailed {
			log.Infof("Withdraw request already rejected, withdraw id: %d", record.ID)
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
		log.Infof("Withdraw request rejected and refund event created, withdraw id: %d, address: %s, refund amount: %s",
			record.ID, record.Address, totalAmount.String())

		return nil
	}); err != nil {
		log.Infof("Failed to reject withdraw request, withdraw id: %d, error: %v", withdrawID, err)
		return err
	}
	return nil
}
