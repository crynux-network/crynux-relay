package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

type GetDelegationEmissionLineChartInput struct {
	UserAddress string `json:"user_address" path:"user_address" description:"delegator address of the delegation" validate:"required"`
	NodeAddress string `json:"node_address" path:"node_address" description:"node address of the delegation" validate:"required"`
	Network     string `json:"network" query:"network" description:"network of the delegation" validate:"required"`
	Count       *int   `json:"count" query:"count" description:"number of weekly data points"`
}

type GetDelegationEmissionLineChartData struct {
	Timestamps []int64         `json:"timestamps"`
	Emission   []models.BigInt `json:"emission"`
}

type GetDelegationEmissionLineChartOutput struct {
	response.Response
	Data *GetDelegationEmissionLineChartData `json:"data"`
}

func GetDelegationEmissionLineChart(c *gin.Context, input *GetDelegationEmissionLineChartInput) (*GetDelegationEmissionLineChartOutput, error) {
	weeks, err := service.ClampChartWeeks(input.Count)
	if err != nil {
		if errors.Is(err, service.ErrInvalidChartWeeks) {
			return nil, response.NewValidationErrorResponse("count", err.Error())
		}
		return nil, response.NewExceptionResponse(err)
	}

	chartRange, err := service.BuildEmissionChartRange(time.Now().UTC(), config.GetConfig().Dao.MainnetStartTime, weeks)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	details := make([]models.VestingDelegationEmissionDetail, 0)
	if len(chartRange.WeekStarts) > 0 {
		details, err = models.ListVestingDelegationEmissionDetailsByUserNodeNetworkAndStartTimeRange(
			c.Request.Context(),
			config.GetDB(),
			input.UserAddress,
			input.NodeAddress,
			input.Network,
			chartRange.RangeStart,
			chartRange.RangeEnd,
		)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	timestamps, emission := service.BuildDelegationEmissionIncomeSeries(details, chartRange)
	return &GetDelegationEmissionLineChartOutput{
		Data: &GetDelegationEmissionLineChartData{
			Timestamps: timestamps,
			Emission:   emission,
		},
	}, nil
}
