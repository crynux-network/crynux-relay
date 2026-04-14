package client

import (
	"crynux_relay/api/v1/middleware"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
)

type GetClientIncomeStatsInput struct {
	Address string `path:"address" json:"address" form:"address" validate:"required" description:"The address of the user"`
	End     *int64 `json:"end" query:"end" description:"end timestamp"`
	Count   *int   `json:"count" query:"count" description:"number of data points"`
}

type ClientIncomeStatsData struct {
	Timestamps             []int64         `json:"timestamps"`
	NodeIncome             []models.BigInt `json:"node_income"`
	DelegatedStakingIncome []models.BigInt `json:"delegated_staking_income"`
}

type GetClientIncomeStatsResponse struct {
	response.Response
	Data *ClientIncomeStatsData `json:"data"`
}

func GetClientIncomeStats(c *gin.Context, input *GetClientIncomeStatsInput) (*GetClientIncomeStatsResponse, error) {
	address := middleware.GetUserAddress(c)

	if address != input.Address {
		validationErr := response.NewValidationErrorResponse("address", "Address mismatch")
		return nil, validationErr
	}

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
	var nodeIncomes []models.BigInt
	var delegatedStakingIncomes []models.BigInt
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)

		if ne, ok := nodeEarningsMap[timestamp]; ok {
			nodeIncomes = append(nodeIncomes, ne.OperatorEarning)
		} else {
			nodeIncomes = append(nodeIncomes, models.BigInt{Int: *big.NewInt(0)})
		}

		if ue, ok := userEarningsMap[timestamp]; ok {
			delegatedStakingIncomes = append(delegatedStakingIncomes, ue.Earning)
		} else {
			delegatedStakingIncomes = append(delegatedStakingIncomes, models.BigInt{Int: *big.NewInt(0)})
		}
	}

	return &GetClientIncomeStatsResponse{
		Data: &ClientIncomeStatsData{
			Timestamps:             timestamps,
			NodeIncome:             nodeIncomes,
			DelegatedStakingIncome: delegatedStakingIncomes,
		},
	}, nil
}
