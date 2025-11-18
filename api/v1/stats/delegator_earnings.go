package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
)

type GetDelegatorEarningsLineChartInput struct {
	Address string `json:"address" path:"address" description:"delegator address" validate:"required"`
	End     *int64 `json:"end" query:"end" description:"end timestamp"`
	Count   *int   `json:"count" query:"count" description:"number of data points"`
}

type GetDelegatorEarningsLineChartData struct {
	Timestamps []int64         `json:"timestamps"`
	Earnings   []models.BigInt `json:"earnings"`
}

type GetDelegatorEarningsLineChartOutput struct {
	response.Response
	Data *GetDelegatorEarningsLineChartData `json:"data"`
}

func GetDelegatorEarningsLineChart(c *gin.Context, input *GetDelegatorEarningsLineChartInput) (*GetDelegatorEarningsLineChartOutput, error) {
	end := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	if input.End != nil {
		end = time.Unix(*input.End, 0).Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	count := 30
	if input.Count != nil {
		count = *input.Count
	}
	start := end.Add(-time.Duration(count) * 24 * time.Hour)

	userEarnings, err := models.GetUserEarnings(c.Request.Context(), config.GetDB(), input.Address, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	userEarningsMap := make(map[int64]models.UserEarning)
	for _, ue := range userEarnings {
		if ue.Time.Valid {
			userEarningsMap[ue.Time.Time.Truncate(24*time.Hour).Unix()] = ue
		}
	}

	var timestamps []int64
	var earnings []models.BigInt
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)

		if ue, ok := userEarningsMap[timestamp]; ok {
			earnings = append(earnings, ue.Earning)
		} else {
			earnings = append(earnings, models.BigInt{Int: *big.NewInt(0)})
		}
	}

	return &GetDelegatorEarningsLineChartOutput{
		Data: &GetDelegatorEarningsLineChartData{
			Timestamps: timestamps,
			Earnings:   earnings,
		},
	}, nil
}
