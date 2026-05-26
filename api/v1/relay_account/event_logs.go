package relayaccount

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const emptyEventPayload = "{}"

type GetRelayAccountEventLogsInput struct {
	StartID uint `query:"start_id" json:"start_id" description:"Start ID"`
	Limit   int  `query:"limit" json:"limit" description:"Limit"`
}

type GetRelayAccountEventLogsInputWithSignature struct {
	GetRelayAccountEventLogsInput
	Timestamp int64  `query:"timestamp" json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" json:"signature" description:"Signature" validate:"required"`
}

type RelayAccountEventLog struct {
	ID        uint                         `json:"id"`
	CreatedAt uint64                       `json:"created_at"`
	Address   string                       `json:"address"`
	Amount    string                       `json:"amount"`
	Type      models.RelayAccountEventType `json:"type"`
	Payload   string                       `json:"payload"`
}

type GetRelayAccountEventLogsResponse struct {
	response.Response
	Data []RelayAccountEventLog `json:"data"`
}

func marshalEventPayload(payload interface{}) (string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(payloadJSON), nil
}

func buildEventPayload(event models.RelayAccountEvent, vestingMap map[uint]models.VestingRecord) (string, error) {
	switch event.Type {
	case models.RelayAccountEventTypeDeposit:
		reason := event.Reason
		reasons := strings.SplitN(reason, "-", 3)
		if len(reasons) != 3 || reasons[0] != strconv.Itoa(int(models.RelayAccountEventTypeDeposit)) {
			return "", fmt.Errorf("invalid deposit event reason: event_id=%d", event.ID)
		}
		payload := map[string]string{
			"tx_hash": reasons[1],
			"network": reasons[2],
		}
		return marshalEventPayload(payload)
	case models.RelayAccountEventTypeVestingCreated:
		vestingID, ok := models.ParseVestingCreatedReason(event.Reason)
		if !ok {
			return "", fmt.Errorf("invalid vesting created event reason: event_id=%d", event.ID)
		}
		record, ok := vestingMap[vestingID]
		if !ok {
			return "", fmt.Errorf("vesting record not found for event payload: event_id=%d vesting_id=%d", event.ID, vestingID)
		}
		payload := models.BuildVestingCreatedPayload(record)
		return marshalEventPayload(payload)
	case models.RelayAccountEventTypeVestingRelease:
		vestingID, _, _, ok := models.ParseVestingReleaseReason(event.Reason)
		if !ok {
			return "", fmt.Errorf("invalid vesting release event reason: event_id=%d", event.ID)
		}
		payload := map[string]interface{}{
			"vesting_id": vestingID,
		}
		return marshalEventPayload(payload)
	default:
		return emptyEventPayload, nil
	}
}

func loadVestingRecordsForEvents(ctx context.Context, events []models.RelayAccountEvent) (map[uint]models.VestingRecord, error) {
	vestingIDSet := make(map[uint]struct{})
	for _, event := range events {
		if event.Type != models.RelayAccountEventTypeVestingCreated || event.Status != models.RelayAccountEventStatusProcessed {
			continue
		}
		vestingID, ok := models.ParseVestingCreatedReason(event.Reason)
		if !ok {
			return nil, fmt.Errorf("invalid vesting created event reason: event_id=%d", event.ID)
		}
		vestingIDSet[vestingID] = struct{}{}
	}
	if len(vestingIDSet) == 0 {
		return map[uint]models.VestingRecord{}, nil
	}
	vestingIDs := make([]uint, 0, len(vestingIDSet))
	for vestingID := range vestingIDSet {
		vestingIDs = append(vestingIDs, vestingID)
	}
	var records []models.VestingRecord
	if err := config.GetDB().WithContext(ctx).
		Model(&models.VestingRecord{}).
		Where("id IN (?)", vestingIDs).
		Find(&records).Error; err != nil {
		return nil, err
	}
	vestingMap := make(map[uint]models.VestingRecord, len(records))
	for _, record := range records {
		vestingMap[record.ID] = record
	}
	return vestingMap, nil
}

func GetRelayAccountEventLogs(c *gin.Context, in *GetRelayAccountEventLogsInputWithSignature) (*GetRelayAccountEventLogsResponse, error) {
	match, address, err := validate.ValidateSignature(in.GetRelayAccountEventLogsInput, in.Timestamp, in.Signature)
	if err != nil || !match {
		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	if address != config.GetConfig().Withdraw.RelayWalletAddress {
		validationErr := response.NewValidationErrorResponse("address", "Invalid address")
		return nil, validationErr
	}

	var events []models.RelayAccountEvent
	dbCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := config.GetDB().WithContext(dbCtx).
		Model(&models.RelayAccountEvent{}).
		Where("id > ?", in.StartID).
		Where("status != ?", models.RelayAccountEventStatusInvalid).
		Order("id ASC").
		Limit(in.Limit).
		Find(&events).Error; err != nil {
		return nil, err
	}

	logs := make([]RelayAccountEventLog, 0, len(events))
	if len(events) > 0 {
		appConfig := config.GetConfig()
		invalidIDs := make([]uint, 0)
		vestingMap, err := loadVestingRecordsForEvents(dbCtx, events)
		if err != nil {
			return nil, err
		}
		for _, event := range events {
			if event.Status == models.RelayAccountEventStatusPending {
				break
			}

			if event.Status != models.RelayAccountEventStatusProcessed {
				continue
			}

			valid := utils.VerifyMAC([]byte(event.Reason), appConfig.MAC.SecretKey, event.MAC)
			if !valid {
				invalidIDs = append(invalidIDs, event.ID)
				continue
			}

			payload, err := buildEventPayload(event, vestingMap)
			if err != nil {
				return nil, err
			}

			logs = append(logs, RelayAccountEventLog{
				ID:        event.ID,
				CreatedAt: uint64(event.CreatedAt.Unix()),
				Address:   event.Address,
				Amount:    event.Amount.String(),
				Type:      event.Type,
				Payload:   payload,
			})
		}

		if len(invalidIDs) > 0 {
			if err := config.GetDB().WithContext(dbCtx).
				Model(&models.RelayAccountEvent{}).
				Where("id IN (?)", invalidIDs).
				Update("status", models.RelayAccountEventStatusInvalid).Error; err != nil {
				return nil, err
			}
		}
	}

	return &GetRelayAccountEventLogsResponse{
		Data: logs,
	}, nil
}
