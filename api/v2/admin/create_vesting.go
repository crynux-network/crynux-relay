package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateVestingItem struct {
	Address        string `json:"address" validate:"required"`
	TotalAmount    string `json:"total_amount" validate:"required"`
	StartTime      int64  `json:"start_time" validate:"required"`
	DurationDays   uint   `json:"duration_days" validate:"required"`
	Source         string `json:"source" validate:"required"`
	ExternalID     string `json:"external_id" validate:"required"`
	AdminSignature string `json:"admin_signature" validate:"required"`
}

type CreateVestingInput struct {
	Items []CreateVestingItem `json:"items" validate:"required,min=1,dive,required"`
}

type CreateVestingData struct {
	Count uint `json:"count"`
}

type CreateVestingResponse struct {
	response.Response
	Data CreateVestingData `json:"data"`
}

func CreateVestingRecords(c *gin.Context, in *CreateVestingInput) (*CreateVestingResponse, error) {
	inputs := make([]service.CreateVestingRecordInput, 0, len(in.Items))
	for _, item := range in.Items {
		inputs = append(inputs, service.CreateVestingRecordInput{
			Address:        item.Address,
			TotalAmount:    item.TotalAmount,
			StartTime:      item.StartTime,
			DurationDays:   item.DurationDays,
			Source:         item.Source,
			ExternalID:     item.ExternalID,
			AdminSignature: item.AdminSignature,
		})
	}

	created, err := service.CreateVestingRecords(c.Request.Context(), config.GetDB(), inputs)
	if err != nil {
		if errors.Is(err, service.ErrInvalidVestingAddress) ||
			errors.Is(err, service.ErrInvalidVestingAmount) ||
			errors.Is(err, service.ErrInvalidVestingDuration) ||
			errors.Is(err, service.ErrInvalidVestingSource) ||
			errors.Is(err, service.ErrInvalidVestingExternalID) ||
			errors.Is(err, service.ErrInvalidVestingSignature) ||
			errors.Is(err, service.ErrInvalidVestingSigner) ||
			errors.Is(err, service.ErrVestingSignerAddressNotSet) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: "duplicate vesting source and external id"},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}

	return &CreateVestingResponse{
		Data: CreateVestingData{
			Count: uint(len(created)),
		},
	}, nil
}
