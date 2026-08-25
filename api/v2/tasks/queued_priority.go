package tasks

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type QueuedTaskPriorityData struct {
	AsOf                int64          `json:"as_of"`
	QueuedTaskCount     int64          `json:"queued_task_count"`
	HighestPriorityGwei *models.BigInt `json:"highest_priority_gwei"`
	MedianPriorityGwei  *models.BigInt `json:"median_priority_gwei"`
	LowestPriorityGwei  *models.BigInt `json:"lowest_priority_gwei"`
}

type GetQueuedTaskPriorityResponse struct {
	response.Response
	Data QueuedTaskPriorityData `json:"data"`
}

func GetQueuedTaskPriority(c *gin.Context) (*GetQueuedTaskPriorityResponse, error) {
	snapshot := service.GetQueuedTaskPrioritySnapshot()
	return &GetQueuedTaskPriorityResponse{
		Data: QueuedTaskPriorityData{
			AsOf:                snapshot.AsOf,
			QueuedTaskCount:     snapshot.QueuedTaskCount,
			HighestPriorityGwei: snapshot.HighestPriorityGwei,
			MedianPriorityGwei:  snapshot.MedianPriorityGwei,
			LowestPriorityGwei:  snapshot.LowestPriorityGwei,
		},
	}, nil
}
