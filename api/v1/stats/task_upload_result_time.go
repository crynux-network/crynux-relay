package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"time"

	"github.com/gin-gonic/gin"
)

type GetTaskUploadResultTimeHistogramInput struct {
	TaskType TaskTypeString `query:"task_type" validate:"required" enum:"Image,Text,All"`
	Period   TimeUnit       `query:"period" validate:"required" enum:"Hour,Day,Week"`
}

type GetTaskUploadResultTimeHistogramData struct {
	UploadResultTimes []int64 `json:"upload_result_times"`
	TaskCounts        []int64 `json:"task_count"`
}

type GetTaskUploadResultTimeHistogramResponse struct {
	response.Response
	Data *GetTaskUploadResultTimeHistogramData `json:"data"`
}

func GetTaskUploadResultTimeHistogram(_ *gin.Context, input *GetTaskUploadResultTimeHistogramInput) (*GetTaskUploadResultTimeHistogramResponse, error) {
	now := time.Now().UTC()
	var start time.Time
	switch input.Period {
	case UnitHour:
		start = now.Truncate(time.Hour).Add(-time.Hour)
	case UnitDay:
		start = now.Truncate(time.Hour).Add(-24 * time.Hour)
	default:
		start = now.Truncate(time.Hour).Add(-7 * 24 * time.Hour)
	}

	timeout := 300
	subQuery := config.GetDB().Model(&models.TaskUploadResultTimeCount{}).Select("seconds, count").Where("start >= ?", start).Where("seconds >= ?", 0).Where("seconds < ?", timeout)
	switch input.TaskType {
	case ImageTaskType:
		subQuery = subQuery.Where("task_type = ?", models.TaskTypeSD)
	case TextTaskType:
		subQuery = subQuery.Where("task_type = ?", models.TaskTypeLLM)
	}

	rows, err := config.GetDB().Table("(?) as s", subQuery).Select("s.seconds AS seconds", "SUM(s.count) AS count").Group("seconds").Order("seconds").Rows()
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	defer rows.Close()

	seconds := make([]int64, 0)
	counts := make([]int64, 0)
	for rows.Next() {
		var second, count int64
		if err := rows.Scan(&second, &count); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
		seconds = append(seconds, second)
		counts = append(counts, count)
	}

	return &GetTaskUploadResultTimeHistogramResponse{
		Data: &GetTaskUploadResultTimeHistogramData{
			UploadResultTimes: seconds,
			TaskCounts:        counts,
		},
	}, nil
}
