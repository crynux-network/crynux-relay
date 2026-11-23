package stats

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type GetTaskFeeHistogramParams struct {
	TaskType TaskTypeString `query:"task_type" validate:"required" enum:"Image,Text,Video,All"`
}

type GetTaskFeeHistogramData struct {
	TaskFees   []string `json:"task_fees"`
	TaskCounts []int64  `json:"task_counts"`
}

type GetTaskFeeHistogramOutput struct {
	Data *GetTaskFeeHistogramData `json:"data"`
}

type taskFeeHistogramCacheEntry struct {
	Data      *GetTaskFeeHistogramData
	ExpiresAt time.Time
}

var taskFeeHistogramCacheTTL = time.Minute
var taskFeeHistogramCacheLock sync.RWMutex
var taskFeeHistogramCache = make(map[TaskTypeString]taskFeeHistogramCacheEntry)

func GetTaskFeeHistogram(_ *gin.Context, input *GetTaskFeeHistogramParams) (*GetTaskFeeHistogramOutput, error) {
	end := time.Now().UTC()
	if data, ok := getTaskFeeHistogramFromCache(input.TaskType, end); ok {
		return &GetTaskFeeHistogramOutput{Data: data}, nil
	}

	start := end.Add(-time.Hour)

	stmt := config.GetDB().Model(&models.InferenceTask{}).Where("created_at >= ?", start).Where("created_at < ?", end).Where("task_fee IS NOT NULL").Where("task_fee > ?", 0)
	switch input.TaskType {
	case ImageTaskType:
		stmt = stmt.Where("task_type IN ?", []models.TaskType{models.TaskTypeSD, models.TaskTypeSDFTLora})
	case TextTaskType:
		stmt = stmt.Where("task_type = ?", models.TaskTypeLLM)
	case VideoTaskType:
		stmt = stmt.Where("task_type = ?", models.TaskTypeVideo)
	}

	rows, err := stmt.Select("task_fee").Rows()
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	defer rows.Close()

	fees := make([]*big.Int, 0, 1024)
	var minFee *big.Int
	var maxFee *big.Int
	for rows.Next() {
		var feeStr string
		if err := rows.Scan(&feeStr); err != nil {
			return nil, response.NewExceptionResponse(err)
		}

		fee := new(big.Int)
		if _, ok := fee.SetString(feeStr, 10); !ok {
			return nil, response.NewExceptionResponse(fmt.Errorf("invalid task_fee value %q", feeStr))
		}

		fees = append(fees, fee)
		if minFee == nil || fee.Cmp(minFee) < 0 {
			minFee = new(big.Int).Set(fee)
		}
		if maxFee == nil || fee.Cmp(maxFee) > 0 {
			maxFee = new(big.Int).Set(fee)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	taskFees := make([]string, 10)
	taskCounts := make([]int64, 10)
	if len(fees) == 0 {
		data := &GetTaskFeeHistogramData{
			TaskFees:   taskFees,
			TaskCounts: taskCounts,
		}
		setTaskFeeHistogramCache(input.TaskType, data, end)
		return &GetTaskFeeHistogramOutput{Data: data}, nil
	}

	var binSize *big.Int
	if minFee.Cmp(maxFee) < 0 {
		feeRange := new(big.Int).Sub(maxFee, minFee)
		binSize = pow10Big(len(feeRange.String()) - 1)
	} else {
		binSize = pow10Big(len(minFee.String()) - 1)
	}

	binStart := new(big.Int).Div(minFee, binSize)
	binStart.Mul(binStart, binSize)

	for i := 0; i < 10; i++ {
		label := new(big.Int).Mul(big.NewInt(int64(i)), binSize)
		label.Add(label, binStart)
		taskFees[i] = label.String()
	}

	for _, fee := range fees {
		offset := new(big.Int).Sub(fee, binStart)
		indexBig := new(big.Int).Div(offset, binSize)
		if !indexBig.IsInt64() {
			taskCounts[9]++
			continue
		}

		index := indexBig.Int64()
		if index < 0 {
			continue
		}
		if index > 9 {
			index = 9
		}
		taskCounts[index]++
	}

	data := &GetTaskFeeHistogramData{
		TaskFees:   taskFees,
		TaskCounts: taskCounts,
	}
	setTaskFeeHistogramCache(input.TaskType, data, end)

	return &GetTaskFeeHistogramOutput{
		Data: data,
	}, nil
}

func getTaskFeeHistogramFromCache(taskType TaskTypeString, now time.Time) (*GetTaskFeeHistogramData, bool) {
	taskFeeHistogramCacheLock.RLock()
	entry, ok := taskFeeHistogramCache[taskType]
	taskFeeHistogramCacheLock.RUnlock()
	if !ok || now.After(entry.ExpiresAt) {
		return nil, false
	}
	return cloneTaskFeeHistogramData(entry.Data), true
}

func setTaskFeeHistogramCache(taskType TaskTypeString, data *GetTaskFeeHistogramData, now time.Time) {
	taskFeeHistogramCacheLock.Lock()
	taskFeeHistogramCache[taskType] = taskFeeHistogramCacheEntry{
		Data:      cloneTaskFeeHistogramData(data),
		ExpiresAt: now.Add(taskFeeHistogramCacheTTL),
	}
	taskFeeHistogramCacheLock.Unlock()
}

func cloneTaskFeeHistogramData(data *GetTaskFeeHistogramData) *GetTaskFeeHistogramData {
	if data == nil {
		return nil
	}
	taskFees := append([]string(nil), data.TaskFees...)
	taskCounts := append([]int64(nil), data.TaskCounts...)
	return &GetTaskFeeHistogramData{
		TaskFees:   taskFees,
		TaskCounts: taskCounts,
	}
}

func pow10Big(exp int) *big.Int {
	if exp <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil)
}
