package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
)

type GetNodeStakingsInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
	End     *int64 `json:"end" query:"end" description:"end timestamp"`
	Count   *int   `json:"count" query:"count" description:"number of data points"`
}

type GetNodeStakingsData struct {
	Timestamps       []int64 `json:"timestamps"`
	OperatorStakings []models.BigInt `json:"operator_stakings"`
	DelegatorStakings []models.BigInt `json:"delegator_stakings"`
	TotalStakings    []models.BigInt `json:"total_stakings"`
}

type GetNodeStakingsOutput struct {
	response.Response
	Data *GetNodeStakingsData `json:"data"`
}

func GetNodeStakingsLineChart(c *gin.Context, input *GetNodeStakingsInput) (*GetNodeStakingsOutput, error) {
	if service.GetDelegatorShare(input.Address) == 0 {
		return nil, response.NewNotFoundErrorResponse()
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

	nodeStakings, err := models.GetNodeStakings(c.Request.Context(), config.GetDB(), input.Address, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeStakingsMap := make(map[int64]models.NodeStaking)
	for _, ns := range nodeStakings {
		t := ns.Time.UTC().Truncate(24 * time.Hour).Unix()
		if oldNs, ok := nodeStakingsMap[t]; !ok {
			nodeStakingsMap[t] = ns
		} else if oldNs.Time.Before(ns.Time) {
			nodeStakingsMap[t] = ns
		}
	}

	var timestamps []int64
	var operatorStakings []models.BigInt
	var delegatorStakings []models.BigInt
	var totalStakings []models.BigInt
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)
		if ns, ok := nodeStakingsMap[timestamp]; ok {
			operatorStakings = append(operatorStakings, models.BigInt{Int: ns.OperatorStaking.Int})
			delegatorStakings = append(delegatorStakings, models.BigInt{Int: ns.DelegatorStaking.Int})
			totalStaking := big.NewInt(0).Add(&ns.OperatorStaking.Int, &ns.DelegatorStaking.Int)
			totalStakings = append(totalStakings, models.BigInt{Int: *totalStaking})
		} else {
			operatorStakings = append(operatorStakings, models.BigInt{Int: *big.NewInt(0)})
			delegatorStakings = append(delegatorStakings, models.BigInt{Int: *big.NewInt(0)})
			totalStakings = append(totalStakings, models.BigInt{Int: *big.NewInt(0)})
		}
	}

	return &GetNodeStakingsOutput{
		Data: &GetNodeStakingsData{
			Timestamps:       timestamps,
			OperatorStakings: operatorStakings,
			DelegatorStakings: delegatorStakings,
			TotalStakings:    totalStakings,
		},
	}, nil
}