package models

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"gorm.io/gorm"
)

type TaskStatus uint8

var ErrTaskIDEmpty = errors.New("InferenceTask.ID is 0")
var ErrTaskStatusChanged = errors.New("InferenceTask.Status changed during update")

const (
	TaskQueued TaskStatus = iota
	TaskStarted
	TaskParametersUploaded
	TaskErrorReported
	TaskScoreReady
	TaskValidated
	TaskGroupValidated
	TaskEndInvalidated
	TaskEndSuccess
	TaskEndAborted
	TaskEndGroupRefund
	TaskEndGroupSuccess
)

type TaskType uint8

const (
	TaskTypeSD TaskType = iota
	TaskTypeLLM
	TaskTypeSDFTLora
)

type TaskAbortReason uint8

const (
	TaskAbortReasonNone TaskAbortReason = iota
	TaskAbortTimeout
	TaskAbortModelDownloadFailed
	TaskAbortIncorrectResult
	TaskAbortTaskFeeTooLow
	TaskAbortGroupTimeout
	TaskAbortErrorReported
	TaskAbortCreatorCancelled
	TaskAbortCreatorValidationTimeout
	TaskAbortResultUploadTimeout
	TaskAbortNodeSlashed
)

type TaskError uint8

const (
	TaskErrorNone TaskError = iota
	TaskErrorParametersValidationFailed
)

type StringArray []string

func (arr *StringArray) Scan(val interface{}) error {
	var arrString string
	switch v := val.(type) {
	case string:
		arrString = v
	case []byte:
		arrString = string(v)
	case nil:
		return nil
	default:
		return errors.New(fmt.Sprint("Unable to parse value to StringArray: ", val))
	}
	*arr = strings.Split(arrString, ";")
	return nil
}

func (arr StringArray) Value() (driver.Value, error) {
	res := strings.Join(arr, ";")
	return res, nil
}

func (arr StringArray) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(arr))
}

func (arr *StringArray) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*[]string)(arr))
}

type InferenceTask struct {
	gorm.Model
	TaskArgs         string     `json:"task_args"`
	TaskIDCommitment string     `json:"task_id_commitment" gorm:"uniqueIndex"`
	Creator          string     `json:"creator"`
	SamplingSeed     string     `json:"sampling_seed"`
	Nonce            string     `json:"nonce"`
	Status           TaskStatus `json:"status"`
	TaskType         TaskType   `json:"task_type" gorm:"index"`
	TaskVersion      string     `json:"task_version"`
	ModelName        string     `json:"model_name" gorm:"size:512;not null;default:''"`
	ModelVariant     string     `json:"model_variant" gorm:"size:191;not null;default:''"`
	RequestedDType   string     `json:"requested_dtype" gorm:"column:requested_dtype;size:64;not null;default:'auto'"`
	ExecutionDType   string     `json:"execution_dtype" gorm:"column:execution_dtype;size:64;not null;default:''"`
	QuantizeBits     uint64     `json:"quantize_bits" gorm:"not null;default:0"`
	Timeout          uint64     `json:"timeout"`
	MinVRAM          uint64     `json:"min_vram"`
	RequiredGPU      string     `json:"required_gpu"`
	RequiredGPUVRAM  uint64     `json:"required_gpu_vram"`
	TaskFee          BigInt     `json:"task_fee"`
	// Priority orders queued tasks for dispatch: task_fee divided by the
	// VRAM-weighted estimated node seconds, floored to an integer.
	Priority                BigInt          `json:"priority" gorm:"type:decimal(65,0);not null;default:0"`
	EstimatedNodeSeconds    float64         `json:"estimated_node_seconds" gorm:"not null;default:0"`
	VRAMWeight              float64         `json:"vram_weight" gorm:"column:vram_weight;not null;default:0"`
	PricingUnits            float64         `json:"pricing_units" gorm:"not null;default:0"`
	SDUnits                 *uint64         `json:"sd_units" gorm:"type:bigint unsigned;null;default:null"`
	LLMInputBytes           *uint64         `json:"llm_input_bytes" gorm:"type:bigint unsigned;null;default:null"`
	LLMTextInputBytes       *uint64         `json:"llm_text_input_bytes" gorm:"type:bigint unsigned;null;default:null"`
	LLMImageCount           *uint64         `json:"llm_image_count" gorm:"type:bigint unsigned;null;default:null"`
	LLMImagePixels          *uint64         `json:"llm_image_pixels" gorm:"type:bigint unsigned;null;default:null"`
	LLMMaxNewTokens         *uint64         `json:"llm_max_new_tokens" gorm:"type:bigint unsigned;null;default:null"`
	TaskSize                uint64          `json:"task_size"`
	ModelIDs                StringArray     `json:"model_ids" gorm:"type:text"`
	AbortReason             TaskAbortReason `json:"abort_reason"`
	TaskError               TaskError       `json:"task_error"`
	Score                   string          `json:"score" gorm:"type:text"`
	QOSScore                sql.NullInt64   `json:"qos_score"`
	SelectedNode            string          `json:"selected_node"`
	ExecutionGPU            string          `json:"execution_gpu" gorm:"column:execution_gpu;size:191;not null;default:''"`
	ExecutionGPUVRAM        uint64          `json:"execution_gpu_vram" gorm:"column:execution_gpu_vram;not null;default:0"`
	TaskID                  string          `json:"task_id"`
	ModelSwtiched           bool            `json:"model_swtiched"`
	EstimatedCompletionTime sql.NullTime    `json:"estimated_completion_time" gorm:"null;default:null"`
	DeadlineAt              sql.NullTime    `json:"deadline_at" gorm:"null;default:null"`
	// time when task is created (get from blockchain)
	CreateTime sql.NullTime `json:"create_time" gorm:"index;null;default:null"`
	// time when task is started (get from blockchain)
	StartTime sql.NullTime `json:"start_time" gorm:"index;null;default:null"`
	// time when the selected node fetched the task for the first time
	DeliveredTime sql.NullTime `json:"delivered_time" gorm:"null;default:null"`
	// time when task score is ready (get from blockchain)
	ScoreReadyTime sql.NullTime `json:"score_ready_time" gorm:"index;null;default:null"`
	// time when relay find that task score is validated
	ValidatedTime sql.NullTime `json:"validated_time" gorm:"index;null;default:null"`
	// time when relay report task results are uploaded
	ResultUploadedTime sql.NullTime `json:"result_uploaded_time" gorm:"index;null;default:null"`
}

func (task *InferenceTask) VersionNumbers() [3]uint64 {
	taskVersions := strings.Split(task.TaskVersion, ".")
	if len(taskVersions) != 3 {
		log.Fatalf("Task version is invalid: %d", task.ID)
	}
	taskMajorVersion, err := strconv.ParseUint(taskVersions[0], 10, 64)
	if err != nil {
		log.Fatalf("Task version is invalid: %d", task.ID)
	}
	taskMinorVersion, err := strconv.ParseUint(taskVersions[1], 10, 64)
	if err != nil {
		log.Fatalf("Task version is invalid: %d", task.ID)
	}
	taskPatchVersion, err := strconv.ParseUint(taskVersions[2], 10, 64)
	if err != nil {
		log.Fatalf("Task version is invalid: %d", task.ID)
	}
	return [3]uint64{taskMajorVersion, taskMinorVersion, taskPatchVersion}
}

func (task *InferenceTask) SyncStatus(ctx context.Context, db *gorm.DB) error {
	if task.ID == 0 {
		return ErrTaskIDEmpty
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var res InferenceTask
	if err := db.WithContext(dbCtx).Model(task).Select("status", "abort_reason").First(&res, task.ID).Error; err != nil {
		return err
	}
	task.Status = res.Status
	task.AbortReason = res.AbortReason
	return nil
}

func (task *InferenceTask) Create(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Create(task).Error; err != nil {
		return err
	}
	return nil
}

func (task *InferenceTask) Update(ctx context.Context, db *gorm.DB, values map[string]interface{}) error {
	if task.ID == 0 {
		return ErrTaskIDEmpty
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result *gorm.DB
	if _, ok := values["status"]; ok {
		result = db.WithContext(dbCtx).Model(task).Where("status = ?", task.Status).Updates(values)
		if result.RowsAffected == 0 {
			return ErrTaskStatusChanged
		}
	} else {
		result = db.WithContext(dbCtx).Model(task).Updates(values)
	}
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

func GetTaskByIDCommitment(ctx context.Context, db *gorm.DB, taskIDCommitment string) (*InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	task := InferenceTask{TaskIDCommitment: taskIDCommitment}
	if err := db.WithContext(dbCtx).Model(&task).Where(&task).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func GetTasksByCreatorAndCommitments(ctx context.Context, db *gorm.DB, creator string, commitments []string) ([]InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var tasks []InferenceTask
	err := db.WithContext(dbCtx).
		Where("creator = ? AND task_id_commitment IN ?", creator, commitments).
		Find(&tasks).Error
	return tasks, err
}

func GetTaskGroupByTaskID(ctx context.Context, db *gorm.DB, taskID string) ([]InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var tasks []InferenceTask
	if err := db.WithContext(dbCtx).Model(&InferenceTask{}).Where("task_id = ?", taskID).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (task *InferenceTask) ExecutionTime() time.Duration {
	if task.StartTime.Valid && task.ScoreReadyTime.Valid {
		return task.ScoreReadyTime.Time.Sub(task.StartTime.Time)
	}
	return time.Duration(1<<63 - 1)
}

func GetTotalTaskCount(ctx context.Context, db *gorm.DB) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type result struct {
		Count int64 `json:"count"`
	}

	var res result
	if err := db.WithContext(dbCtx).Model(&InferenceTask{}).Select("max(id) as count").First(&res).Error; err != nil {
		return 0, err
	}
	return res.Count, nil
}

func GetRunningTaskCount(ctx context.Context, db *gorm.DB) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&InferenceTask{}).
		Where("status IN ?", []TaskStatus{TaskStarted, TaskParametersUploaded, TaskErrorReported, TaskScoreReady, TaskValidated, TaskGroupValidated}).
		Count(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}

func GetQueuedTaskCount(ctx context.Context, db *gorm.DB) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&InferenceTask{}).Where("status = ?", TaskQueued).Count(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}

// QueuedTaskPriorityRange is the highest, median, and lowest priority among
// all TaskQueued rows ordered by priority DESC, id ASC.
type QueuedTaskPriorityRange struct {
	Count   int64
	Highest *BigInt
	Median  *BigInt
	Lowest  *BigInt
}

func GetQueuedTaskPriorityRange(ctx context.Context, db *gorm.DB) (*QueuedTaskPriorityRange, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int64
	if err := db.WithContext(dbCtx).Model(&InferenceTask{}).Where("status = ?", TaskQueued).Count(&count).Error; err != nil {
		return nil, err
	}
	result := &QueuedTaskPriorityRange{Count: count}
	if count == 0 {
		return result, nil
	}

	readPriorityAtOffset := func(offset int) (*BigInt, error) {
		var task InferenceTask
		err := db.WithContext(dbCtx).Model(&InferenceTask{}).
			Select("priority").
			Where("status = ?", TaskQueued).
			Order("priority DESC, id ASC").
			Offset(offset).
			Limit(1).
			Take(&task).Error
		if err != nil {
			return nil, err
		}
		return &BigInt{Int: *new(big.Int).Set(&task.Priority.Int)}, nil
	}

	highest, err := readPriorityAtOffset(0)
	if err != nil {
		return nil, err
	}
	median, err := readPriorityAtOffset(int(count / 2))
	if err != nil {
		return nil, err
	}
	lowest, err := readPriorityAtOffset(int(count - 1))
	if err != nil {
		return nil, err
	}

	result.Highest = highest
	result.Median = median
	result.Lowest = lowest
	return result, nil
}

func GetTimeoutAbortedNodeCount(ctx context.Context, db *gorm.DB, since time.Time) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var res int64
	if err := db.WithContext(dbCtx).Model(&InferenceTask{}).
		Select("count(distinct selected_node)").
		Where("status = ?", TaskEndAborted).
		Where("abort_reason IN ?", []TaskAbortReason{TaskAbortTimeout, TaskAbortResultUploadTimeout}).
		Where("selected_node <> ?", "").
		Where("updated_at >= ?", since).
		Find(&res).Error; err != nil {
		return 0, err
	}
	return res, nil
}
