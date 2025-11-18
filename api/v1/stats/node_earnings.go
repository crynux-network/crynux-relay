package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
)

type GetNodeEarningsLineChartInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
	End     *int64 `json:"end" query:"end" description:"end timestamp"`
	Count   *int   `json:"count" query:"count" description:"number of data points"`
}

type GetNodeEarningsLineChartData struct {
	Timestamps        []int64         `json:"timestamps"`
	OperatorEarnings  []models.BigInt `json:"operator_earnings"`
	DelegatorEarnings []models.BigInt `json:"delegator_earnings"`
	TotalEarnings     []models.BigInt `json:"total_earnings"`
}

type GetNodeEarningsLineChartOutput struct {
	response.Response
	Data *GetNodeEarningsLineChartData `json:"data"`
}

func GetNodeEarningsLineChart(c *gin.Context, input *GetNodeEarningsLineChartInput) (*GetNodeEarningsLineChartOutput, error) {
	end := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	if input.End != nil {
		end = time.Unix(*input.End, 0).Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	count := 30
	if input.Count != nil {
		count = *input.Count
	}
	start := end.Add(-time.Duration(count) * 24 * time.Hour)

	nodeEarnings, err := models.GetNodeEarnings(c.Request.Context(), config.GetDB(), input.Address, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeEarningsMap := make(map[int64]models.NodeEarning)
	for _, ne := range nodeEarnings {
		if ne.Time.Valid {
			nodeEarningsMap[ne.Time.Time.Truncate(24*time.Hour).Unix()] = ne
		}
	}

	var timestamps []int64
	var operatorEarnings []models.BigInt
	var delegatorEarnings []models.BigInt
	var totalEarnings []models.BigInt
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)
		if ne, ok := nodeEarningsMap[timestamp]; ok {
			operatorEarnings = append(operatorEarnings, ne.OperatorEarning)
			delegatorEarnings = append(delegatorEarnings, ne.DelegatorEarning)
			totalEarnings = append(totalEarnings, models.BigInt{Int: *big.NewInt(0).Add(&ne.OperatorEarning.Int, &ne.DelegatorEarning.Int)})
		} else {
			operatorEarnings = append(operatorEarnings, models.BigInt{Int: *big.NewInt(0)})
			delegatorEarnings = append(delegatorEarnings, models.BigInt{Int: *big.NewInt(0)})
			totalEarnings = append(totalEarnings, models.BigInt{Int: *big.NewInt(0)})
		}
	}

	return &GetNodeEarningsLineChartOutput{
		Data: &GetNodeEarningsLineChartData{
			Timestamps:        timestamps,
			OperatorEarnings:  operatorEarnings,
			DelegatorEarnings: delegatorEarnings,
			TotalEarnings:     totalEarnings,
		},
	}, nil
}
