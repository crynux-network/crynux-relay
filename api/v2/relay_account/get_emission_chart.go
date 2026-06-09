package relayaccount

import (
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

type GetEmissionChartInput struct {
	Address string `path:"address" json:"address" description:"Address of account"`
	Weeks   *int   `query:"weeks" description:"Number of completed weeks to return"`
}

type EmissionChartData struct {
	Timestamps     []int64         `json:"timestamps"`
	EmissionIncome []models.BigInt `json:"emission_income"`
}

type GetEmissionChartResponse struct {
	response.Response
	Data *EmissionChartData `json:"data"`
}

func GetEmissionChart(c *gin.Context, in *GetEmissionChartInput) (*GetEmissionChartResponse, error) {
	address := middleware.GetUserAddress(c)
	if address != in.Address {
		return nil, response.NewValidationErrorResponse("address", "Address mismatch")
	}

	weeks, err := service.ClampChartWeeks(in.Weeks)
	if err != nil {
		if errors.Is(err, service.ErrInvalidChartWeeks) {
			return nil, response.NewValidationErrorResponse("weeks", err.Error())
		}
		return nil, response.NewExceptionResponse(err)
	}

	chartRange, err := service.BuildEmissionChartRange(time.Now().UTC(), config.GetConfig().Dao.MainnetStartTime, weeks)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	records := make([]models.VestingRecord, 0)
	if len(chartRange.WeekStarts) > 0 {
		records, err = models.ListVestingRecordsByAddressAndStartTimeRange(
			c.Request.Context(),
			config.GetDB(),
			in.Address,
			chartRange.RangeStart,
			chartRange.RangeEnd,
		)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	timestamps, emissionIncome := service.BuildEmissionIncomeSeries(records, chartRange)
	return &GetEmissionChartResponse{
		Data: &EmissionChartData{
			Timestamps:     timestamps,
			EmissionIncome: emissionIncome,
		},
	}, nil
}
