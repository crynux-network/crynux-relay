package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
)

type GetTaskSuccessRateLineChartParams struct {
	TaskType TaskTypeString `query:"task_type" validate:"required" enum:"Image,Text,Video,All"`
	Period   TimeUnit       `query:"period" validate:"required" enum:"Hour,Day,Week"`
}

type GetTaskSuccessRateLineChartData struct {
	Timestamps   []int64   `json:"timestamps"`
	SuccessRates []float64 `json:"success_rate"`
}

type GetTaskSuccessRateLineChartOutput struct {
	response.Response
	Data *GetTaskSuccessRateLineChartData `json:"data"`
}

func GetTaskSuccessRateLineChart(_ *gin.Context, input *GetTaskSuccessRateLineChartParams) (*GetTaskSuccessRateLineChartOutput, error) {
	timestampSuccessCount := make(map[int64]int64)
	timestampTotalCount := make(map[int64]int64)

	now := time.Now().UTC()
	var start, end time.Time
	var duration time.Duration
	switch input.Period {
	case UnitHour:
		duration = time.Hour
		start = now.Truncate(duration).Add(-24 * duration)
		end = now.Truncate(duration)
	case UnitDay:
		duration = 24 * time.Hour
		start = now.Truncate(duration).Add(-15 * duration)
		end = now.Truncate(duration)
	default:
		duration = 7 * 24 * time.Hour
		start = now.Truncate(duration).Add(-8 * duration)
		end = now.Truncate(duration)
	}

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
		timestampTotalCount[timestamp] += taskCount.TotalCount
		timestampSuccessCount[timestamp] += taskCount.SuccessCount
	}

	timestamps := make([]int64, 0)
	for timestamp := range timestampTotalCount {
		timestamps = append(timestamps, timestamp)
	}

	slices.Sort(timestamps)

	successRates := make([]float64, 0)
	for _, timestamp := range timestamps {
		var rate float64 = 0
		if timestampTotalCount[timestamp] > 0 {
			rate = float64(timestampSuccessCount[timestamp]) / float64(timestampTotalCount[timestamp])
		}
		successRates = append(successRates, rate)
	}

	return &GetTaskSuccessRateLineChartOutput{
		Data: &GetTaskSuccessRateLineChartData{
			Timestamps:   timestamps,
			SuccessRates: successRates,
		},
	}, nil
}
