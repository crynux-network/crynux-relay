package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetNodeEmissionChartInput struct {
	Address string `path:"address" json:"address" description:"node address" validate:"required"`
	Weeks   *int   `query:"weeks" description:"Number of completed weeks to return"`
}

type NodeEmissionChartData struct {
	Timestamps         []int64         `json:"timestamps"`
	NodeEmissionIncome []models.BigInt `json:"node_emission_income"`
}

type GetNodeEmissionChartOutput struct {
	response.Response
	Data *NodeEmissionChartData `json:"data"`
}

func hasNodeEmissionAccess(node *models.Node) bool {
	if node == nil {
		return false
	}
	if node.DelegatorShare == 0 {
		return false
	}
	return service.GetDelegatorShare(node.Address, node.Network) > 0
}

func GetNodeEmissionChart(c *gin.Context, input *GetNodeEmissionChartInput) (*GetNodeEmissionChartOutput, error) {
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), input.Address)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, response.NewAccessDeniedErrorResponse()
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	if !hasNodeEmissionAccess(node) {
		return nil, response.NewAccessDeniedErrorResponse()
	}

	weeks, err := service.ClampChartWeeks(input.Weeks)
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
		records, err = models.ListVestingRecordsByAddressAndTypeAndStartTimeRange(
			c.Request.Context(),
			config.GetDB(),
			node.Address,
			models.VestingTypeNode,
			chartRange.RangeStart,
			chartRange.RangeEnd,
		)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	timestamps, emissionIncome := service.BuildEmissionIncomeSeries(records, chartRange)
	return &GetNodeEmissionChartOutput{
		Data: &NodeEmissionChartData{
			Timestamps:         timestamps,
			NodeEmissionIncome: emissionIncome,
		},
	}, nil
}
