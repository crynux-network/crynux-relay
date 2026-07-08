package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidVestingAddress          = errors.New("invalid vesting address")
	ErrInvalidVestingAmount           = errors.New("invalid vesting amount")
	ErrInvalidVestingDuration         = errors.New("invalid vesting duration")
	ErrInvalidVestingSignature        = errors.New("invalid vesting signature")
	ErrInvalidVestingSigner           = errors.New("invalid vesting signer")
	ErrInvalidVestingType             = errors.New("invalid vesting type")
	ErrInvalidVestingDelegationDetail = errors.New("invalid vesting delegation detail")
	ErrVestingSignerAddressNotSet     = errors.New("vesting signer address not set")
	ErrVestingRecordNotFound          = errors.New("vesting record not found")
	ErrVestingReleaseRangeInvalid     = errors.New("vesting release range invalid")
	ErrVestingReleaseExceedsSchedule  = errors.New("vesting release exceeds schedule")
)

type CreateVestingDelegationDetailInput struct {
	NodeAddress    string `json:"node_address"`
	Network        string `json:"network"`
	TaskFee        string `json:"task_fee"`
	EmissionAmount string `json:"emission_amount"`
	StartTime      int64  `json:"start_time"`
}

type CreateVestingRecordInput struct {
	Address           string                               `json:"address"`
	TotalAmount       string                               `json:"total_amount"`
	StartTime         int64                                `json:"start_time"`
	DurationDays      uint                                 `json:"duration_days"`
	Type              string                               `json:"type"`
	AdminSignature    string                               `json:"admin_signature"`
	DelegationDetails []CreateVestingDelegationDetailInput `json:"delegation_details"`
}

type vestingSignPayload struct {
	Address      string `json:"address"`
	TotalAmount  string `json:"total_amount"`
	StartTime    int64  `json:"start_time"`
	DurationDays uint   `json:"duration_days"`
	Type         string `json:"type"`
}

func isValidUint256Amount(amount *big.Int) bool {
	return amount != nil && amount.Sign() > 0 && amount.BitLen() <= 256
}

func buildVestingSignMessage(payload vestingSignPayload) string {
	return fmt.Sprintf(
		"Crynux Relay Vesting\nAddress: %s\nTotalAmount: %s\nStartTime: %d\nDurationDays: %d\nType: %s",
		payload.Address,
		payload.TotalAmount,
		payload.StartTime,
		payload.DurationDays,
		payload.Type,
	)
}

func isValidVestingType(vestingType string) bool {
	switch vestingType {
	case models.VestingTypeNode, models.VestingTypeDelegation, models.VestingTypeOther:
		return true
	default:
		return false
	}
}

func normalizeVestingInput(input CreateVestingRecordInput) (vestingSignPayload, *big.Int, error) {
	verifier := blockchain.NewSignatureVerifier()
	if err := verifier.ValidateAddress(input.Address); err != nil {
		return vestingSignPayload{}, nil, ErrInvalidVestingAddress
	}
	if input.DurationDays == 0 {
		return vestingSignPayload{}, nil, ErrInvalidVestingDuration
	}
	normalizedType := strings.TrimSpace(input.Type)
	if !isValidVestingType(normalizedType) {
		return vestingSignPayload{}, nil, ErrInvalidVestingType
	}
	amount, ok := big.NewInt(0).SetString(strings.TrimSpace(input.TotalAmount), 10)
	if !ok || !isValidUint256Amount(amount) {
		return vestingSignPayload{}, nil, ErrInvalidVestingAmount
	}
	payload := vestingSignPayload{
		Address:      input.Address,
		TotalAmount:  amount.String(),
		StartTime:    input.StartTime,
		DurationDays: input.DurationDays,
		Type:         normalizedType,
	}
	return payload, amount, nil
}

func verifyVestingCreationSignature(input CreateVestingRecordInput, payload vestingSignPayload) error {
	if strings.TrimSpace(input.AdminSignature) == "" {
		return ErrInvalidVestingSignature
	}
	conf := config.GetConfig()
	if strings.TrimSpace(conf.Admin.VestingSignerAddress) == "" {
		return ErrVestingSignerAddressNotSet
	}

	verifier := blockchain.NewSignatureVerifier()
	if err := verifier.ValidateSignatureFormat(input.AdminSignature); err != nil {
		return ErrInvalidVestingSignature
	}

	message := buildVestingSignMessage(payload)
	recoveredSigner, err := verifier.RecoverAddress(message, input.AdminSignature)
	if err != nil {
		return ErrInvalidVestingSignature
	}
	if !strings.EqualFold(recoveredSigner, conf.Admin.VestingSignerAddress) {
		return ErrInvalidVestingSigner
	}
	return nil
}

func normalizeVestingDelegationDetails(input CreateVestingRecordInput, payload vestingSignPayload, totalAmount *big.Int) ([]models.VestingDelegationEmissionDetail, error) {
	if payload.Type != models.VestingTypeDelegation {
		if len(input.DelegationDetails) > 0 {
			return nil, ErrInvalidVestingDelegationDetail
		}
		return nil, nil
	}
	if len(input.DelegationDetails) == 0 {
		return nil, ErrInvalidVestingDelegationDetail
	}

	verifier := blockchain.NewSignatureVerifier()
	details := make([]models.VestingDelegationEmissionDetail, 0, len(input.DelegationDetails))
	sum := big.NewInt(0)
	seenDetailKeys := make(map[string]struct{}, len(input.DelegationDetails))
	for _, detail := range input.DelegationDetails {
		if err := verifier.ValidateAddress(detail.NodeAddress); err != nil {
			return nil, ErrInvalidVestingDelegationDetail
		}
		network := strings.TrimSpace(detail.Network)
		if network == "" {
			return nil, ErrInvalidVestingDelegationDetail
		}
		if detail.StartTime != payload.StartTime {
			return nil, ErrInvalidVestingDelegationDetail
		}
		detailKey := strings.Join([]string{strings.ToLower(payload.Address), strings.ToLower(detail.NodeAddress), network, fmt.Sprint(detail.StartTime)}, "|")
		if _, ok := seenDetailKeys[detailKey]; ok {
			return nil, ErrInvalidVestingDelegationDetail
		}
		seenDetailKeys[detailKey] = struct{}{}
		taskFee, ok := big.NewInt(0).SetString(strings.TrimSpace(detail.TaskFee), 10)
		if !ok || !isValidUint256Amount(taskFee) {
			return nil, ErrInvalidVestingDelegationDetail
		}
		emissionAmount, ok := big.NewInt(0).SetString(strings.TrimSpace(detail.EmissionAmount), 10)
		if !ok || !isValidUint256Amount(emissionAmount) {
			return nil, ErrInvalidVestingDelegationDetail
		}
		sum.Add(sum, emissionAmount)
		details = append(details, models.VestingDelegationEmissionDetail{
			UserAddress:    payload.Address,
			NodeAddress:    detail.NodeAddress,
			Network:        network,
			TaskFee:        models.BigInt{Int: *taskFee},
			EmissionAmount: models.BigInt{Int: *emissionAmount},
			StartTime:      time.Unix(detail.StartTime, 0).UTC(),
		})
	}
	if sum.Cmp(totalAmount) != 0 {
		return nil, ErrInvalidVestingDelegationDetail
	}
	return details, nil
}

func accumulateDelegationEmissionWeeklyTotals(totals map[string]models.NodeDelegationEmissionWeeklyIncrement, details []models.VestingDelegationEmissionDetail) {
	for _, detail := range details {
		key := strings.Join([]string{strings.ToLower(detail.NodeAddress), fmt.Sprint(detail.StartTime.Unix())}, "|")
		increment, ok := totals[key]
		if !ok {
			increment = models.NodeDelegationEmissionWeeklyIncrement{
				NodeAddress:    detail.NodeAddress,
				StartTime:      detail.StartTime,
				EmissionAmount: big.NewInt(0),
			}
		}
		increment.EmissionAmount.Add(increment.EmissionAmount, &detail.EmissionAmount.Int)
		totals[key] = increment
	}
}

func CreateVestingRecords(ctx context.Context, db *gorm.DB, inputs []CreateVestingRecordInput) ([]models.VestingRecord, error) {
	if len(inputs) == 0 {
		return []models.VestingRecord{}, nil
	}

	created := make([]models.VestingRecord, 0, len(inputs))
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		delegationEmissionTotals := make(map[string]models.NodeDelegationEmissionWeeklyIncrement)
		for _, input := range inputs {
			payload, amount, err := normalizeVestingInput(input)
			if err != nil {
				return err
			}
			if err := verifyVestingCreationSignature(input, payload); err != nil {
				return err
			}
			details, err := normalizeVestingDelegationDetails(input, payload, amount)
			if err != nil {
				return err
			}
			record := models.VestingRecord{
				Address:        payload.Address,
				TotalAmount:    models.BigInt{Int: *amount},
				ReleasedAmount: models.BigInt{Int: *big.NewInt(0)},
				StartTime:      time.Unix(payload.StartTime, 0).UTC(),
				DurationDays:   payload.DurationDays,
				Type:           payload.Type,
				AdminSignature: input.AdminSignature,
				Status:         models.VestingStatusActive,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			for i := range details {
				details[i].VestingRecordID = record.ID
			}
			if len(details) > 0 {
				if err := tx.Create(&details).Error; err != nil {
					return err
				}
				accumulateDelegationEmissionWeeklyTotals(delegationEmissionTotals, details)
			}
			if err := createVestingCreatedRelayAccountEvent(ctx, tx, record); err != nil {
				return err
			}
			created = append(created, record)
		}
		increments := make([]models.NodeDelegationEmissionWeeklyIncrement, 0, len(delegationEmissionTotals))
		for _, increment := range delegationEmissionTotals {
			increments = append(increments, increment)
		}
		if err := models.UpsertNodeDelegationEmissionWeeklyTotalIncrements(ctx, tx, increments); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	refreshedNodeAddresses := make(map[string]struct{})
	for _, record := range created {
		if _, ok := refreshedNodeAddresses[record.Address]; ok {
			continue
		}
		if err := RefreshNodeScoreStake(ctx, db, record.Address, now); err != nil {
			return nil, err
		}
		refreshedNodeAddresses[record.Address] = struct{}{}
	}
	return created, nil
}

func ProcessDueVestingReleases(ctx context.Context, db *gorm.DB, now time.Time, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 100
	}

	pending, err := hasPendingVestingReleaseEvent(ctx, db)
	if err != nil {
		return err
	}
	if pending {
		log.Warn("Skip vesting release because pending vesting release events exist")
		return nil
	}

	startID := uint(0)
	for {
		var records []models.VestingRecord
		if err := db.WithContext(ctx).
			Model(&models.VestingRecord{}).
			Where("status = ?", models.VestingStatusActive).
			Where("slashed = ?", false).
			Where("id > ?", startID).
			Order("id ASC").
			Limit(batchSize).
			Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}

		for _, record := range records {
			startID = record.ID
			if err := processSingleVestingRelease(ctx, db, record.ID, now); err != nil {
				return err
			}
		}
	}
}

func hasPendingVestingReleaseEvent(ctx context.Context, db *gorm.DB) (bool, error) {
	var event models.RelayAccountEvent
	if err := db.WithContext(ctx).
		Model(&models.RelayAccountEvent{}).
		Select("id").
		Where("type = ?", models.RelayAccountEventTypeVestingRelease).
		Where("status = ?", models.RelayAccountEventStatusPending).
		Limit(1).
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func processSingleVestingRelease(ctx context.Context, db *gorm.DB, vestingID uint, now time.Time) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record models.VestingRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&models.VestingRecord{}).
			Where("id = ?", vestingID).
			First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrVestingRecordNotFound
			}
			return err
		}
		if record.Status != models.VestingStatusActive {
			return nil
		}
		if record.Slashed {
			return nil
		}

		shouldReleased := models.ComputeVestingShouldReleased(&record.TotalAmount.Int, record.StartTime, record.DurationDays, now)
		if shouldReleased.Cmp(&record.ReleasedAmount.Int) <= 0 {
			if (&record.ReleasedAmount.Int).Cmp(&record.TotalAmount.Int) >= 0 {
				return tx.Model(&models.VestingRecord{}).Where("id = ?", record.ID).
					Update("status", models.VestingStatusCompleted).Error
			}
			return nil
		}
		if shouldReleased.Cmp(&record.TotalAmount.Int) > 0 {
			return ErrVestingReleaseExceedsSchedule
		}

		fromReleased := new(big.Int).Set(&record.ReleasedAmount.Int)
		toReleased := new(big.Int).Set(shouldReleased)
		commitFunc, err := releaseVestingToRelayAccount(ctx, tx, record.ID, record.Address, fromReleased, toReleased)
		if err != nil {
			return err
		}

		newStatus := models.VestingStatusActive
		if toReleased.Cmp(&record.TotalAmount.Int) == 0 {
			newStatus = models.VestingStatusCompleted
		}
		updates := map[string]interface{}{
			"released_amount": models.BigInt{Int: *toReleased},
			"status":          newStatus,
		}
		if err := tx.Model(&models.VestingRecord{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
			return err
		}
		return commitFunc()
	})
}

func GetAddressLockedVestingAmount(ctx context.Context, db *gorm.DB, address string, now time.Time) (*big.Int, error) {
	var records []models.VestingRecord
	if err := db.WithContext(ctx).
		Model(&models.VestingRecord{}).
		Where("address = ?", address).
		Where("status = ?", models.VestingStatusActive).
		Where("slashed = ?", false).
		Find(&records).Error; err != nil {
		return nil, err
	}

	totalLocked := big.NewInt(0)
	for _, record := range records {
		totalLocked.Add(totalLocked, record.LockedAmountAt(now))
	}
	return totalLocked, nil
}

func SlashNodeVestingsTx(ctx context.Context, tx *gorm.DB, nodeAddress string) (bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := tx.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Where("address = ?", nodeAddress).
		Where("status = ?", models.VestingStatusActive).
		Where("slashed = ?", false).
		Update("slashed", true)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func SlashNodeVestings(ctx context.Context, db *gorm.DB, nodeAddress string, now time.Time) error {
	var refresh bool
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		refresh, err = SlashNodeVestingsTx(ctx, tx, nodeAddress)
		return err
	}); err != nil {
		return err
	}
	if refresh {
		return RefreshNodeScoreStake(ctx, db, nodeAddress, now)
	}
	return nil
}

func RestoreNodeVestings(ctx context.Context, db *gorm.DB, nodeAddress string, now time.Time) error {
	verifier := blockchain.NewSignatureVerifier()
	if err := verifier.ValidateAddress(nodeAddress); err != nil {
		return ErrInvalidVestingAddress
	}

	var refresh bool
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		result := tx.WithContext(dbCtx).
			Model(&models.VestingRecord{}).
			Where("address = ?", nodeAddress).
			Where("slashed = ?", true).
			Update("slashed", false)
		if result.Error != nil {
			return result.Error
		}
		refresh = result.RowsAffected > 0
		return nil
	}); err != nil {
		return err
	}
	if refresh {
		return RefreshNodeScoreStake(ctx, db, nodeAddress, now)
	}
	return nil
}
