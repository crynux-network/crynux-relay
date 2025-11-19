package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"time"

	"github.com/gin-gonic/gin"
)

type GetNodeDelegatorNumInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
	End     *int64 `json:"end" query:"end" description:"end timestamp"`
	Count   *int   `json:"count" query:"count" description:"number of data points"`
}

type GetNodeDelegatorNumData struct {
	Timestamps     []int64 `json:"timestamps"`
	DelegatorNums  []uint64 `json:"delegator_nums"`
}

type GetNodeDelegatorNumOutput struct {
	response.Response
	Data *GetNodeDelegatorNumData `json:"data"`
}

func GetNodeDelegatorNumLineChart(c *gin.Context, input *GetNodeDelegatorNumInput) (*GetNodeDelegatorNumOutput, error) {
	end := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	if input.End != nil {
		end = time.Unix(*input.End, 0).Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
	count := 30
	if input.Count != nil {
		count = *input.Count
	}
	start := end.Add(-time.Duration(count) * 24 * time.Hour)

	nodeDelegatorCounts, err := models.GetNodeDelegatorCounts(c.Request.Context(), config.GetDB(), input.Address, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeDelegatorCountsMap := make(map[int64]models.NodeDelegatorCount)
	for _, ndc := range nodeDelegatorCounts {
		t := ndc.Time.UTC().Truncate(24 * time.Hour).Unix()
		if _, ok := nodeDelegatorCountsMap[t]; ok {
			if ndc.Count > nodeDelegatorCountsMap[t].Count {
				nodeDelegatorCountsMap[t] = ndc
			}
		} else {
			nodeDelegatorCountsMap[t] = ndc
		}
	}

	var timestamps []int64
	var delegatorNums []uint64
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)

		if ndc, ok := nodeDelegatorCountsMap[timestamp]; ok {
			delegatorNums = append(delegatorNums, ndc.Count)
		} else {
			delegatorNums = append(delegatorNums, 0)
		}
	}

	return &GetNodeDelegatorNumOutput{
		Data: &GetNodeDelegatorNumData{
			Timestamps:    timestamps,
			DelegatorNums: delegatorNums,
		},
	}, nil
}