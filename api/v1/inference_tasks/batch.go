package inference_tasks

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"crynux_relay/service"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type BatchCreateInput struct {
	Items []TaskInput `json:"items" validate:"required"`
}

type BatchCreateInputWithSignature struct {
	BatchCreateInput
	Timestamp int64  `json:"timestamp" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

type BatchCreateItemResult struct {
	TaskIDCommitment string            `json:"task_id_commitment"`
	Outcome          string            `json:"outcome"`
	Sequence         uint64            `json:"sequence,omitempty"`
	SamplingSeed     string            `json:"sampling_seed,omitempty"`
	Status           models.TaskStatus `json:"status"`
	Error            string            `json:"error,omitempty"`
}

type BatchCreateResponse struct {
	response.Response
	Data []BatchCreateItemResult `json:"data"`
}

type BatchStatusInput struct {
	TaskIDCommitments []string `json:"task_id_commitments" validate:"required"`
}

type BatchStatusInputWithSignature struct {
	BatchStatusInput
	Timestamp int64  `json:"timestamp" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

type BatchStatusItem struct {
	TaskIDCommitment        string                 `json:"task_id_commitment"`
	Found                   bool                   `json:"found"`
	Status                  models.TaskStatus      `json:"status"`
	AbortReason             models.TaskAbortReason `json:"abort_reason"`
	TaskError               models.TaskError       `json:"task_error"`
	Sequence                uint64                 `json:"sequence,omitempty"`
	SamplingSeed            string                 `json:"sampling_seed,omitempty"`
	ExecutionGPU            string                 `json:"execution_gpu,omitempty"`
	ExecutionGPUVRAM        uint64                 `json:"execution_gpu_vram,omitempty"`
	EstimatedCompletionTime *time.Time             `json:"estimated_completion_time,omitempty"`
	ResultAvailable         bool                   `json:"result_available"`
}

type BatchStatusResponse struct {
	response.Response
	Data []BatchStatusItem `json:"data"`
}

type BatchValidateInput struct {
	Items []ValidateTaskInput `json:"items" validate:"required"`
}

type BatchValidateInputWithSignature struct {
	BatchValidateInput
	Timestamp int64  `json:"timestamp" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

type BatchMutationItemResult struct {
	TaskIDCommitments []string `json:"task_id_commitments,omitempty"`
	TaskIDCommitment  string   `json:"task_id_commitment,omitempty"`
	Outcome           string   `json:"outcome"`
	Error             string   `json:"error,omitempty"`
}

type BatchMutationResponse struct {
	response.Response
	Data []BatchMutationItemResult `json:"data"`
}

type BatchAbortItem struct {
	TaskIDCommitment string                 `json:"task_id_commitment" validate:"required"`
	AbortReason      models.TaskAbortReason `json:"abort_reason" validate:"required"`
}

type BatchAbortInput struct {
	Items []BatchAbortItem `json:"items" validate:"required"`
}

type BatchAbortInputWithSignature struct {
	BatchAbortInput
	Timestamp int64  `json:"timestamp" validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

func validateBatchSignature(data interface{}, timestamp int64, signature string) (string, error) {
	match, address, err := validate.ValidateSignature(data, timestamp, signature)
	if err != nil {
		log.Debugf("batch signature validation failed: %v", err)
	}
	if err != nil || !match {
		return "", response.NewValidationErrorResponse("signature", "Invalid signature")
	}
	return address, nil
}

func validateBatchSize(field string, count, limit int) error {
	if count == 0 {
		return response.NewValidationErrorResponse(field, "Batch must contain at least one item")
	}
	if count > limit {
		return response.NewValidationErrorResponse(field, fmt.Sprintf("Batch exceeds maximum item count %d", limit))
	}
	return nil
}

func recordBatchResponseBytes(endpoint string, value interface{}) {
	encoded, err := json.Marshal(value)
	if err == nil {
		metrics.TaskBatchResponseBytes.WithLabelValues(endpoint).Add(float64(len(encoded)))
	}
}

func rejectDuplicateStrings(field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return response.NewValidationErrorResponse(field, "Duplicate item identity")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func checkBatchCreatorAllowed(ctx context.Context, address string) error {
	if !config.GetConfig().Task.TaskWhitelistEnabled {
		return nil
	}
	allowed, err := service.IsTaskCreatorWhitelisted(ctx, config.GetDB(), address)
	if err != nil {
		return err
	}
	if !allowed {
		return response.NewValidationErrorResponse("address", "Signer not allowed")
	}
	return nil
}

func prepareBatchTask(in TaskInput, address string) (*models.InferenceTask, error) {
	if in.TaskType != models.TaskTypeSD && in.TaskType != models.TaskTypeLLM {
		return nil, errors.New("batch creation supports only ordinary SD and LLM tasks")
	}
	if in.MinVram == nil || in.TaskSize == nil {
		return nil, errors.New("min_vram and task_size are required")
	}
	if in.TaskIDCommitment == "" || in.TaskArgs == "" || in.Nonce == "" ||
		len(in.TaskModelIDs) == 0 || in.TaskVersion == "" {
		return nil, errors.New("required task input is missing")
	}
	if validationErr, err := models.ValidateTaskArgsJsonStr(in.TaskArgs, in.TaskType); err != nil {
		return nil, err
	} else if validationErr != nil {
		return nil, validationErr
	}
	normalizedArgs, err := models.NormalizeTaskArgsModelNames(in.TaskArgs, in.TaskType)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(in.TaskVersion, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid task version")
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return nil, errors.New("invalid task version")
		}
	}
	requiredGPU := ""
	if in.RequiredGPU != nil {
		requiredGPU = models.NormalizeGPUName(*in.RequiredGPU)
	}
	requiredGPUVRAM := uint64(0)
	if in.RequiredGPUVram != nil {
		requiredGPUVRAM = *in.RequiredGPUVram
	}
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	return &models.InferenceTask{
		TaskArgs:         normalizedArgs,
		TaskIDCommitment: in.TaskIDCommitment,
		Creator:          address,
		SamplingSeed:     hexutil.Encode(seed),
		Nonce:            in.Nonce,
		Status:           models.TaskQueued,
		TaskType:         in.TaskType,
		TaskVersion:      in.TaskVersion,
		MinVRAM:          *in.MinVram,
		RequiredGPU:      requiredGPU,
		RequiredGPUVRAM:  requiredGPUVRAM,
		TaskFee:          in.TaskFee,
		TaskSize:         *in.TaskSize,
		ModelIDs:         models.NormalizeModelIDs(in.TaskModelIDs),
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	}, nil
}

func immutableTaskInputEqual(existing, candidate *models.InferenceTask) bool {
	return existing.Creator == candidate.Creator &&
		existing.TaskArgs == candidate.TaskArgs &&
		existing.Nonce == candidate.Nonce &&
		existing.TaskType == candidate.TaskType &&
		existing.TaskVersion == candidate.TaskVersion &&
		existing.MinVRAM == candidate.MinVRAM &&
		existing.RequiredGPU == candidate.RequiredGPU &&
		existing.RequiredGPUVRAM == candidate.RequiredGPUVRAM &&
		existing.TaskFee.Cmp(&candidate.TaskFee.Int) == 0 &&
		existing.TaskSize == candidate.TaskSize &&
		slices.Equal([]string(existing.ModelIDs), []string(candidate.ModelIDs))
}

func createBatchItem(ctx context.Context, in TaskInput, address string) BatchCreateItemResult {
	result := BatchCreateItemResult{TaskIDCommitment: in.TaskIDCommitment}
	candidate, err := prepareBatchTask(in, address)
	if err != nil {
		result.Outcome, result.Error = "permanent_error", err.Error()
		return result
	}
	existing, err := models.GetTaskByIDCommitment(ctx, config.GetDB(), candidate.TaskIDCommitment)
	if err == nil {
		if immutableTaskInputEqual(existing, candidate) {
			result.Outcome = "already_exists"
			result.Sequence = uint64(existing.ID)
			result.SamplingSeed = existing.SamplingSeed
			result.Status = existing.Status
		} else {
			result.Outcome = "commitment_conflict"
		}
		return result
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Outcome, result.Error = "temporary_error", err.Error()
		return result
	}
	if err := service.CreateTask(ctx, config.GetDB(), candidate); err != nil {
		existing, lookupErr := models.GetTaskByIDCommitment(ctx, config.GetDB(), candidate.TaskIDCommitment)
		if lookupErr == nil {
			if immutableTaskInputEqual(existing, candidate) {
				result.Outcome = "already_exists"
				result.Sequence = uint64(existing.ID)
				result.SamplingSeed = existing.SamplingSeed
				result.Status = existing.Status
			} else {
				result.Outcome = "commitment_conflict"
			}
			return result
		}
		if errors.Is(err, service.ErrInsufficientRelayAccount) {
			result.Outcome, result.Error = "permanent_error", err.Error()
		} else {
			result.Outcome, result.Error = "temporary_error", err.Error()
		}
		return result
	}
	result.Outcome = "created"
	result.Sequence = uint64(candidate.ID)
	result.SamplingSeed = candidate.SamplingSeed
	result.Status = candidate.Status
	return result
}

func CreateTaskBatch(c *gin.Context, in *BatchCreateInputWithSignature) (*BatchCreateResponse, error) {
	started := time.Now()
	succeeded := false
	defer func() {
		metrics.TaskBatchDurationSeconds.WithLabelValues("create").Observe(time.Since(started).Seconds())
		result := "failure"
		if succeeded {
			result = "success"
		}
		metrics.TaskBatchRequests.WithLabelValues("create", result).Inc()
	}()
	if err := validateBatchSize("items", len(in.Items), config.GetConfig().Task.BatchCreateMaxItems); err != nil {
		return nil, err
	}
	commitments := make([]string, len(in.Items))
	for i := range in.Items {
		commitments[i] = in.Items[i].TaskIDCommitment
	}
	if err := rejectDuplicateStrings("items", commitments); err != nil {
		return nil, err
	}
	address, err := validateBatchSignature(in.BatchCreateInput, in.Timestamp, in.Signature)
	if err != nil {
		return nil, err
	}
	if err := checkBatchCreatorAllowed(c.Request.Context(), address); err != nil {
		var validationError *response.ValidationErrorResponse
		if errors.As(err, &validationError) {
			return nil, validationError
		}
		return nil, response.NewExceptionResponse(err)
	}
	results := make([]BatchCreateItemResult, len(in.Items))
	for i := range in.Items {
		results[i] = createBatchItem(c.Request.Context(), in.Items[i], address)
		metrics.TaskBatchItems.WithLabelValues("create", results[i].Outcome).Inc()
	}
	out := &BatchCreateResponse{Data: results}
	recordBatchResponseBytes("create", out)
	succeeded = true
	return out, nil
}

func GetTaskBatchStatus(c *gin.Context, in *BatchStatusInputWithSignature) (*BatchStatusResponse, error) {
	started := time.Now()
	succeeded := false
	defer func() {
		metrics.TaskBatchDurationSeconds.WithLabelValues("status").Observe(time.Since(started).Seconds())
		result := "failure"
		if succeeded {
			result = "success"
		}
		metrics.TaskBatchRequests.WithLabelValues("status", result).Inc()
	}()
	if err := validateBatchSize("task_id_commitments", len(in.TaskIDCommitments), config.GetConfig().Task.BatchStatusMaxItems); err != nil {
		return nil, err
	}
	if err := rejectDuplicateStrings("task_id_commitments", in.TaskIDCommitments); err != nil {
		return nil, err
	}
	address, err := validateBatchSignature(in.BatchStatusInput, in.Timestamp, in.Signature)
	if err != nil {
		return nil, err
	}
	tasks, err := models.GetTasksByCreatorAndCommitments(c.Request.Context(), config.GetDB(), address, in.TaskIDCommitments)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	byCommitment := make(map[string]models.InferenceTask, len(tasks))
	for i := range tasks {
		byCommitment[tasks[i].TaskIDCommitment] = tasks[i]
	}
	results := make([]BatchStatusItem, len(in.TaskIDCommitments))
	for i, commitment := range in.TaskIDCommitments {
		results[i].TaskIDCommitment = commitment
		task, ok := byCommitment[commitment]
		if !ok {
			metrics.TaskBatchItems.WithLabelValues("status", "not_found").Inc()
			continue
		}
		results[i].Found = true
		results[i].Status = task.Status
		results[i].AbortReason = task.AbortReason
		results[i].TaskError = task.TaskError
		results[i].Sequence = uint64(task.ID)
		results[i].SamplingSeed = task.SamplingSeed
		results[i].ExecutionGPU = task.ExecutionGPU
		results[i].ExecutionGPUVRAM = task.ExecutionGPUVRAM
		if task.EstimatedCompletionTime.Valid {
			value := task.EstimatedCompletionTime.Time
			results[i].EstimatedCompletionTime = &value
		}
		results[i].ResultAvailable = task.Status == models.TaskEndSuccess || task.Status == models.TaskEndGroupSuccess
		metrics.TaskBatchItems.WithLabelValues("status", "found").Inc()
	}
	out := &BatchStatusResponse{Data: results}
	recordBatchResponseBytes("status", out)
	succeeded = true
	return out, nil
}

func validationIdentity(item ValidateTaskInput) string {
	commitments := slices.Clone(item.TaskIDCommitments)
	slices.Sort(commitments)
	return strings.Join(commitments, "\x00")
}

func validateBatchUnit(ctx context.Context, item ValidateTaskInput, address string) BatchMutationItemResult {
	result := BatchMutationItemResult{TaskIDCommitments: item.TaskIDCommitments}
	if len(item.TaskIDCommitments) != 1 && len(item.TaskIDCommitments) != 3 {
		result.Outcome, result.Error = "permanent_error", "task_id_commitments length must be 1 or 3"
		return result
	}
	if err := rejectDuplicateStrings("task_id_commitments", item.TaskIDCommitments); err != nil {
		result.Outcome, result.Error = "permanent_error", "duplicate task commitment"
		return result
	}
	tasks := make([]*models.InferenceTask, 0, len(item.TaskIDCommitments))
	for _, commitment := range item.TaskIDCommitments {
		task, err := models.GetTaskByIDCommitment(ctx, config.GetDB(), commitment)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result.Outcome, result.Error = "permanent_error", "task not found"
			} else {
				result.Outcome, result.Error = "temporary_error", err.Error()
			}
			return result
		}
		if task.Creator != address {
			result.Outcome, result.Error = "permanent_error", "task not found"
			return result
		}
		tasks = append(tasks, task)
	}
	var err error
	if len(tasks) == 1 {
		err = service.ValidateSingleTask(ctx, tasks[0], item.TaskID, item.VrfProof, item.PublicKey)
	} else {
		err = service.ValidateTaskGroup(ctx, tasks, item.TaskID, item.VrfProof, item.PublicKey)
	}
	switch {
	case err == nil:
		result.Outcome = "validated"
	case errors.Is(err, service.ErrValidationAlreadyApplied):
		result.Outcome = "already_applied"
	case errors.Is(err, models.ErrTaskStatusChanged), errors.Is(err, models.ErrNodeStatusChanged):
		result.Outcome, result.Error = "temporary_error", err.Error()
	default:
		result.Outcome, result.Error = "permanent_error", err.Error()
	}
	return result
}

func ValidateTaskBatch(c *gin.Context, in *BatchValidateInputWithSignature) (*BatchMutationResponse, error) {
	started := time.Now()
	succeeded := false
	defer func() {
		metrics.TaskBatchDurationSeconds.WithLabelValues("validate").Observe(time.Since(started).Seconds())
		result := "failure"
		if succeeded {
			result = "success"
		}
		metrics.TaskBatchRequests.WithLabelValues("validate", result).Inc()
	}()
	if err := validateBatchSize("items", len(in.Items), config.GetConfig().Task.BatchValidateMaxItems); err != nil {
		return nil, err
	}
	identities := make([]string, len(in.Items))
	for i := range in.Items {
		if err := rejectDuplicateStrings("items", in.Items[i].TaskIDCommitments); err != nil {
			return nil, err
		}
		identities[i] = validationIdentity(in.Items[i])
	}
	if err := rejectDuplicateStrings("items", identities); err != nil {
		return nil, err
	}
	address, err := validateBatchSignature(in.BatchValidateInput, in.Timestamp, in.Signature)
	if err != nil {
		return nil, err
	}
	results := make([]BatchMutationItemResult, len(in.Items))
	for i := range in.Items {
		results[i] = validateBatchUnit(c.Request.Context(), in.Items[i], address)
		metrics.TaskBatchItems.WithLabelValues("validate", results[i].Outcome).Inc()
	}
	out := &BatchMutationResponse{Data: results}
	recordBatchResponseBytes("validate", out)
	succeeded = true
	return out, nil
}

func abortBatchItem(ctx context.Context, item BatchAbortItem, address string) BatchMutationItemResult {
	result := BatchMutationItemResult{TaskIDCommitment: item.TaskIDCommitment}
	if item.AbortReason != models.TaskAbortCreatorCancelled {
		result.Outcome, result.Error = "permanent_error", "abort reason not allowed"
		return result
	}
	task, err := models.GetTaskByIDCommitment(ctx, config.GetDB(), item.TaskIDCommitment)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Outcome = "not_found"
		} else {
			result.Outcome, result.Error = "temporary_error", err.Error()
		}
		return result
	}
	if task.Creator != address {
		result.Outcome = "not_found"
		return result
	}
	if task.Status == models.TaskEndAborted && task.AbortReason == models.TaskAbortCreatorCancelled {
		result.Outcome = "already_cancelled"
		return result
	}
	if task.Status != models.TaskQueued {
		result.Outcome = "not_cancellable"
		return result
	}
	task.AbortReason = item.AbortReason
	if err := service.SetTaskStatusEndAborted(ctx, config.GetDB(), task, address); err != nil {
		if errors.Is(err, models.ErrTaskStatusChanged) {
			current, reloadErr := models.GetTaskByIDCommitment(ctx, config.GetDB(), item.TaskIDCommitment)
			if reloadErr == nil && current.Status == models.TaskEndAborted && current.AbortReason == models.TaskAbortCreatorCancelled {
				result.Outcome = "already_cancelled"
			} else if reloadErr == nil {
				result.Outcome = "not_cancellable"
			} else {
				result.Outcome, result.Error = "temporary_error", err.Error()
			}
		} else {
			result.Outcome, result.Error = "temporary_error", err.Error()
		}
		return result
	}
	result.Outcome = "cancelled"
	return result
}

func AbortTaskBatch(c *gin.Context, in *BatchAbortInputWithSignature) (*BatchMutationResponse, error) {
	started := time.Now()
	succeeded := false
	defer func() {
		metrics.TaskBatchDurationSeconds.WithLabelValues("abort").Observe(time.Since(started).Seconds())
		result := "failure"
		if succeeded {
			result = "success"
		}
		metrics.TaskBatchRequests.WithLabelValues("abort", result).Inc()
	}()
	if err := validateBatchSize("items", len(in.Items), config.GetConfig().Task.BatchAbortMaxItems); err != nil {
		return nil, err
	}
	identities := make([]string, len(in.Items))
	for i := range in.Items {
		identities[i] = in.Items[i].TaskIDCommitment
	}
	if err := rejectDuplicateStrings("items", identities); err != nil {
		return nil, err
	}
	address, err := validateBatchSignature(in.BatchAbortInput, in.Timestamp, in.Signature)
	if err != nil {
		return nil, err
	}
	results := make([]BatchMutationItemResult, len(in.Items))
	for i := range in.Items {
		results[i] = abortBatchItem(c.Request.Context(), in.Items[i], address)
		metrics.TaskBatchItems.WithLabelValues("abort", results[i].Outcome).Inc()
	}
	out := &BatchMutationResponse{Data: results}
	recordBatchResponseBytes("abort", out)
	succeeded = true
	return out, nil
}
