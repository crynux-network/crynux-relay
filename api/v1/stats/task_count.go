package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
)

type GetTaskCountLineChartParams struct {
	TaskType TaskTypeString `query:"task_type" validate:"required" enum:"Image,Text,Video,All"`
	Period   TimeUnit       `query:"period" validate:"required" enum:"Hour,Day,Week"`
	End    *int64   `query:"end"`
	Count  *int     `query:"count"`
}

type GetTaskCountLineChartData struct {
	Timestamps []int64 `json:"timestamps"`
	Counts     []int64 `json:"counts"`
}

type GetTaskCountLineChartResponse struct {
	response.Response
	Data *GetTaskCountLineChartData `json:"data"`
}

func GetTaskCountLineChart(_ *gin.Context, input *GetTaskCountLineChartParams) (*GetTaskCountLineChartResponse, error) {
	timestampCounts := make(map[int64]int64)

	now := time.Now().UTC()
	var start, end time.Time
	var duration time.Duration
	var count int
	switch input.Period {
	case UnitHour:
		duration = time.Hour
		count = 24
		if input.Count != nil {
			count = *input.Count
		}
	case UnitDay:
		duration = 24 * time.Hour
		count = 15
		if input.Count != nil {
			count = *input.Count
		}
	default:
		duration = 7 * 24 * time.Hour
		count = 8
		if input.Count != nil {
			count = *input.Count
		}
	}
	if input.End != nil {
		end = time.Unix(*input.End, 0).Truncate(duration)
	} else {
		end = now.Truncate(duration)
	}
	start = end.Add(-time.Duration(count) * duration)


	var allTaskCounts []models.TaskCount
	stmt := config.GetDB().Model(&models.TaskCount{}).Where("start >= ?", start).Where("start < ?", end)
	switch input.TaskType {
	case ImageTaskType:
		stmt = stmt.Where("task_type IN ?", []models.TaskType{models.TaskTypeSD, models.TaskTypeSDFTLora})
	case TextTaskType:
		stmt = stmt.Where("task_type = ?", models.TaskTypeLLM)
	case VideoTaskType:
		stmt = stmt.Where("task_type = ?", models.TaskTypeVideo)
	}
	stmt = stmt.Order("id")

	offset := 0
	for {
		var taskCounts []models.TaskCount
		if err := stmt.Offset(offset).Limit(1000).Find(&taskCounts).Error; err != nil {
			return nil, response.NewExceptionResponse(err)
		}

		allTaskCounts = append(allTaskCounts, taskCounts...)
		if len(taskCounts) < 1000 {
			break
		}
		offset += 1000
	}

	for _, taskCount := range allTaskCounts {
		timestamp := taskCount.Start.Truncate(duration).Unix()
		timestampCounts[timestamp] += taskCount.TotalCount
	}

	timestamps := make([]int64, 0)
	for timestamp := range timestampCounts {
		timestamps = append(timestamps, timestamp)
	}

	slices.Sort(timestamps)

	counts := make([]int64, 0)
	for _, timestamp := range timestamps {
		counts = append(counts, timestampCounts[timestamp])
	}

	return &GetTaskCountLineChartResponse{
		Data: &GetTaskCountLineChartData{
			Timestamps: timestamps,
			Counts:     counts,
		},
	}, nil
}
