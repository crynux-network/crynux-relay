package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
)

type GetDelegationEarningsLineChartInput struct {
	UserAddress string `json:"user_address" path:"user_address" description:"delegator address of the delegation" validate:"required"`
	NodeAddress string `json:"node_address" path:"node_address" description:"node address of the delegation" validate:"required"`
	Network     string `json:"network" query:"network" description:"network of the delegation" validate:"required"`
	End         *int64 `json:"end" query:"end" description:"end timestamp"`
	Count       *int   `json:"count" query:"count" description:"number of data points"`
}

type GetDelegationEarningsLineChartData struct {
	Timestamps []int64         `json:"timestamps"`
	Earnings   []models.BigInt `json:"earnings"`
}

type GetDelegationEarningsLineChartOutput struct {
	response.Response
	Data *GetDelegationEarningsLineChartData `json:"data"`
}

func GetDelegationEarningsLineChart(c *gin.Context, input *GetDelegationEarningsLineChartInput) (*GetDelegationEarningsLineChartOutput, error) {
	end := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	if input.End != nil {
		end = time.Unix(*input.End, 0).Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	count := 30
	if input.Count != nil {
		count = *input.Count
	}
	start := end.Add(-time.Duration(count) * 24 * time.Hour)

	delegationEarnings, err := models.GetUserStakingEarnings(c.Request.Context(), config.GetDB(), input.UserAddress, input.NodeAddress, input.Network, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	delegationEarningsMap := make(map[int64]models.UserStakingEarning)
	for _, dse := range delegationEarnings {
		if dse.Time.Valid {
			delegationEarningsMap[dse.Time.Time.Truncate(24*time.Hour).Unix()] = dse
		}
	}

	var timestamps []int64
	var earnings []models.BigInt
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)

		if dse, ok := delegationEarningsMap[timestamp]; ok {
			earnings = append(earnings, dse.Earning)
		} else {
			earnings = append(earnings, models.BigInt{Int: *big.NewInt(0)})
		}
	}

	return &GetDelegationEarningsLineChartOutput{
		Data: &GetDelegationEarningsLineChartData{
			Timestamps: timestamps,
			Earnings:   earnings,
		},
	}, nil
}
