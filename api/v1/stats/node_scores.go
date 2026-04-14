package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"time"

	"github.com/gin-gonic/gin"
)

type GetNodeScoresInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
	End     *int64 `json:"end" query:"end" description:"end timestamp"`
	Count   *int   `json:"count" query:"count" description:"number of data points"`
}

type GetNodeScoresData struct {
	Timestamps    []int64   `json:"timestamps"`
	StakingScores []float64 `json:"staking_scores"`
	QoSScores     []float64 `json:"qos_scores"`
	ProbWeights   []float64 `json:"prob_weights"`
}

type GetNodeScoresOutput struct {
	response.Response
	Data *GetNodeScoresData `json:"data"`
}

func GetNodeScoresLineChart(c *gin.Context, input *GetNodeScoresInput) (*GetNodeScoresOutput, error) {
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

	nodeScores, err := models.GetNodeScores(c.Request.Context(), config.GetDB(), input.Address, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	stakingScoresMap := make(map[int64][]float64)
	qosScoresMap := make(map[int64][]float64)
	probWeightsMap := make(map[int64][]float64)
	for _, ns := range nodeScores {
		t := ns.Time.UTC().Truncate(24 * time.Hour).Unix()
		stakingScoresMap[t] = append(stakingScoresMap[t], ns.StakingScore)
		qosScoresMap[t] = append(qosScoresMap[t], ns.QOSScore)
		probWeightsMap[t] = append(probWeightsMap[t], ns.ProbWeight)
	}

	var timestamps []int64
	var stakingScores []float64
	var qosScores []float64
	var probWeights []float64
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		timestamp := t.Unix()
		timestamps = append(timestamps, timestamp)

		if _, ok := stakingScoresMap[timestamp]; ok {
			stakingScoreSum := 0.0
			for _, s := range stakingScoresMap[timestamp] {
				stakingScoreSum += s
			}
			avgStakingScore := stakingScoreSum / float64(len(stakingScoresMap[timestamp]))
			stakingScores = append(stakingScores, avgStakingScore)
		} else {
			stakingScores = append(stakingScores, 0.0)
		}

		if _, ok := qosScoresMap[timestamp]; ok {
			qosScoreSum := 0.0
			for _, s := range qosScoresMap[timestamp] {
				qosScoreSum += s
			}
			avgQoSScore := qosScoreSum / float64(len(qosScoresMap[timestamp]))
			qosScores = append(qosScores, avgQoSScore)
		} else {
			qosScores = append(qosScores, 0.0)
		}

		if _, ok := probWeightsMap[timestamp]; ok {
			probWeightSum := 0.0
			for _, s := range probWeightsMap[timestamp] {
				probWeightSum += s
			}
			avgProbWeight := probWeightSum / float64(len(probWeightsMap[timestamp]))
			probWeights = append(probWeights, avgProbWeight)
		} else {
			probWeights = append(probWeights, 0.0)
		}
	}

	return &GetNodeScoresOutput{
		Data: &GetNodeScoresData{
			Timestamps:    timestamps,
			StakingScores: stakingScores,
			QoSScores:     qosScores,
			ProbWeights:   probWeights,
		},
	}, nil
}