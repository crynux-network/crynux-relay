package withdraw

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type GetWithdrawRequestsInput struct {
	StartID uint `query:"start_id" json:"start_id" description:"Start ID"`
	Limit   int  `query:"limit" json:"limit" description:"Limit"`
}

type GetWithdrawRequestsInputWithSignature struct {
	GetWithdrawRequestsInput
	Timestamp int64  `query:"timestamp" json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" json:"signature" description:"Signature" validate:"required"`
}

type WithdrawRecord struct {
	ID                  uint                  `json:"id"`
	CreatedAt           uint64                `json:"created_at"`
	Address             string                `json:"address"`
	BenefitAddress      string                `json:"benefit_address"`
	Amount              string                `json:"amount"`
	Network             string                `json:"network"`
	Status              models.WithdrawStatus `json:"status"`
	RelayAccountEventID uint                  `json:"relay_account_event_id"`
	WithdrawalFee       string                `json:"withdrawal_fee"`
	Timestamp           int64                 `json:"timestamp"`
	Signature           string                `json:"signature"`
}

type GetWithdrawRequestsResponse struct {
	response.Response
	Data []WithdrawRecord `json:"data"`
}

func withdrawalRecordResponse(record models.WithdrawRecord) WithdrawRecord {
	return WithdrawRecord{
		ID:                  record.ID,
		CreatedAt:           uint64(record.CreatedAt.Unix()),
		Address:             record.Address,
		BenefitAddress:      record.BenefitAddress,
		Amount:              record.Amount.String(),
		Network:             record.Network,
		Status:              record.Status,
		RelayAccountEventID: record.RelayAccountEventID,
		WithdrawalFee:       record.WithdrawalFee.String(),
		Timestamp:           record.Timestamp.Int64,
		Signature:           record.Signature.String,
	}
}

func GetWithdrawRequests(c *gin.Context, in *GetWithdrawRequestsInputWithSignature) (*GetWithdrawRequestsResponse, error) {
	match, address, err := validate.ValidateSignature(in.GetWithdrawRequestsInput, in.Timestamp, in.Signature)

	if err != nil || !match {
		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	if address != config.GetConfig().Withdraw.RelayWalletAddress {
		validationErr := response.NewValidationErrorResponse("address", "Invalid address")
		return nil, validationErr
	}

	var records []models.WithdrawRecord

	dbCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := config.GetDB().WithContext(dbCtx).Model(&models.WithdrawRecord{}).Where("id > ?", in.StartID).Where("local_status != ?", models.WithdrawLocalStatusInvalid).Order("id ASC").Limit(in.Limit).Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]WithdrawRecord, 0, len(records))
	if len(records) > 0 {
		appConfig := config.GetConfig()
		invalidIDs := make([]uint, 0)
		for _, record := range records {
			if record.LocalStatus == models.WithdrawLocalStatusPending {
				break
			}

			if record.LocalStatus == models.WithdrawLocalStatusProcessed {
				valid := utils.VerifyMAC([]byte(record.MACString()), appConfig.MAC.SecretKey, record.MAC)
				if !valid {
					invalidIDs = append(invalidIDs, record.ID)
					continue
				}
				results = append(results, withdrawalRecordResponse(record))
			}
		}

		if len(invalidIDs) > 0 {
			if err := config.GetDB().WithContext(dbCtx).Model(&models.WithdrawRecord{}).Where("id IN (?)", invalidIDs).Update("local_status", models.WithdrawLocalStatusInvalid).Error; err != nil {
				return nil, err
			}
		}
	}

	return &GetWithdrawRequestsResponse{
		Data: results,
	}, nil
}
