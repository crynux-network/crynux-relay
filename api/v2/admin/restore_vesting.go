package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
)

type RestoreVestingInput struct {
	NodeAddress string `json:"node_address" validate:"required"`
}

type RestoreVestingResponse struct {
	response.Response
	Data RestoreVestingData `json:"data"`
}

type RestoreVestingData struct {
	RestoredVestingCount       int64  `json:"restored_vesting_count"`
	RestoredVestingTotalAmount string `json:"restored_vesting_total_amount"`
}

func RestoreNodeVestings(c *gin.Context, in *RestoreVestingInput) (*RestoreVestingResponse, error) {
	db := config.GetDB()
	restoredSummary, err := summarizeRestorableNodeVestings(c, in.NodeAddress)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	err = service.RestoreNodeVestings(c.Request.Context(), db, in.NodeAddress, time.Now().UTC())
	if err != nil {
		if errors.Is(err, service.ErrInvalidVestingAddress) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &RestoreVestingResponse{
		Data: RestoreVestingData{
			RestoredVestingCount:       restoredSummary.Count,
			RestoredVestingTotalAmount: restoredSummary.TotalAmount.String(),
		},
	}, nil
}

type restorableNodeVestingSummary struct {
	Count       int64
	TotalAmount *big.Int
}

func summarizeRestorableNodeVestings(c *gin.Context, nodeAddress string) (*restorableNodeVestingSummary, error) {
	records := make([]models.VestingRecord, 0)
	if err := config.GetDB().WithContext(c.Request.Context()).
		Model(&models.VestingRecord{}).
		Select("total_amount").
		Where("address = ?", nodeAddress).
		Where("slashed = ?", true).
		Find(&records).Error; err != nil {
		return nil, err
	}

	totalAmount := big.NewInt(0)
	for _, record := range records {
		totalAmount.Add(totalAmount, &record.TotalAmount.Int)
	}

	return &restorableNodeVestingSummary{
		Count:       int64(len(records)),
		TotalAmount: totalAmount,
	}, nil
}
