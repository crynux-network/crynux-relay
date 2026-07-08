package admin

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	missingReasonDisabled       = "task_tracing_disabled"
	missingReasonExpired        = "task_trace_record_missing_or_expired"
	missingReasonNotReached     = "lifecycle_step_not_reached"
	missingReasonNotPersisted   = "not_persisted"
	missingReasonProofNotStored = "not_stored_by_trace_policy"
)

type GetTaskTraceInput struct {
	TaskIDCommitment string `path:"task_id_commitment" validate:"required"`
}

type TaskTraceMissingData struct {
	Step   string `json:"step"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type TaskTraceDuration struct {
	Name    string `json:"name"`
	Seconds int64  `json:"seconds"`
}

type TaskTraceStep struct {
	Name      string                 `json:"name"`
	Timestamp *int64                 `json:"timestamp,omitempty"`
	Durations []TaskTraceDuration    `json:"durations,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type TaskTraceTaskData struct {
	TaskIDCommitment   string                 `json:"task_id_commitment"`
	TaskID             string                 `json:"task_id,omitempty"`
	Status             models.TaskStatus      `json:"status"`
	SelectedNode       string                 `json:"selected_node,omitempty"`
	Score              string                 `json:"score,omitempty"`
	TaskError          models.TaskError       `json:"task_error"`
	QOSScore           *int64                 `json:"qos_score,omitempty"`
	AbortReason        models.TaskAbortReason `json:"abort_reason"`
	CreateTime         *int64                 `json:"create_time,omitempty"`
	StartTime          *int64                 `json:"start_time,omitempty"`
	ScoreReadyTime     *int64                 `json:"score_ready_time,omitempty"`
	ValidatedTime      *int64                 `json:"validated_time,omitempty"`
	ResultUploadedTime *int64                 `json:"result_uploaded_time,omitempty"`
}

type TaskTraceEventData struct {
	ID               uint   `json:"id"`
	Type             string `json:"type"`
	NodeAddress      string `json:"node_address,omitempty"`
	TaskIDCommitment string `json:"task_id_commitment,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	Args             string `json:"args,omitempty"`
}

type TaskTraceData struct {
	TaskIDCommitment          string                            `json:"task_id_commitment"`
	TaskID                    string                            `json:"task_id,omitempty"`
	Trace                     []TaskTraceStep                   `json:"trace"`
	PrimaryTask               TaskTraceTaskData                 `json:"primary_task"`
	ValidationGroupTasks      []TaskTraceTaskData               `json:"validation_group_tasks,omitempty"`
	PersistedEvents           []TaskTraceEventData              `json:"persisted_events"`
	MemoryTrace               *service.TaskTraceRecord          `json:"memory_trace,omitempty"`
	CurrentSelectedNode       *models.SlashEvidenceNodeSnapshot `json:"current_selected_node,omitempty"`
	AvailableFromExistingData []string                          `json:"available_from_existing_data"`
	AvailableFromMemory       []string                          `json:"available_from_memory"`
	MissingData               []TaskTraceMissingData            `json:"missing_data"`
}

type GetTaskTraceResponse struct {
	response.Response
	Data TaskTraceData `json:"data"`
}

func GetTaskTrace(c *gin.Context, in *GetTaskTraceInput) (*GetTaskTraceResponse, error) {
	db := config.GetDB()
	ctx := c.Request.Context()
	task, err := models.GetTaskByIDCommitment(ctx, db, in.TaskIDCommitment)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}

	groupTasks, err := getTaskTraceGroupTasks(ctx, db, task)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	events, err := queryTaskTraceEvents(ctx, db, groupTasks)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	now := time.Now().UTC()
	store := service.GetTaskTraceStore()
	memoryTrace, memoryAvailable := store.Get(task.TaskIDCommitment, now)
	if !memoryAvailable && task.TaskID != "" {
		for _, record := range store.GetByTaskID(task.TaskID, now) {
			if record.TaskIDCommitment == task.TaskIDCommitment {
				recordCopy := record
				memoryTrace = &recordCopy
				memoryAvailable = true
				break
			}
		}
	}

	builder := newTaskTraceBuilder(task, groupTasks, events, memoryTrace, store.Enabled(), memoryAvailable)
	data, err := builder.build(ctx, db)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &GetTaskTraceResponse{Data: data}, nil
}

type taskTraceBuilder struct {
	task              *models.InferenceTask
	groupTasks        []models.InferenceTask
	events            []models.Event
	memoryTrace       *service.TaskTraceRecord
	tracingEnabled    bool
	memoryAvailable   bool
	availableStored   map[string]struct{}
	availableMemory   map[string]struct{}
	missingData       []TaskTraceMissingData
	trace             []TaskTraceStep
	eventByType       map[string]models.Event
	eventByCommitment map[string]map[string]models.Event
}

func newTaskTraceBuilder(task *models.InferenceTask, groupTasks []models.InferenceTask, events []models.Event, memoryTrace *service.TaskTraceRecord, tracingEnabled, memoryAvailable bool) *taskTraceBuilder {
	builder := &taskTraceBuilder{
		task:              task,
		groupTasks:        groupTasks,
		events:            events,
		memoryTrace:       memoryTrace,
		tracingEnabled:    tracingEnabled,
		memoryAvailable:   memoryAvailable,
		availableStored:   make(map[string]struct{}),
		availableMemory:   make(map[string]struct{}),
		eventByType:       make(map[string]models.Event),
		eventByCommitment: make(map[string]map[string]models.Event),
	}
	for _, event := range events {
		if _, ok := builder.eventByType[event.Type]; !ok {
			builder.eventByType[event.Type] = event
		}
		if event.TaskIDCommitment == "" {
			continue
		}
		typedEvents, ok := builder.eventByCommitment[event.TaskIDCommitment]
		if !ok {
			typedEvents = make(map[string]models.Event)
			builder.eventByCommitment[event.TaskIDCommitment] = typedEvents
		}
		if _, ok := typedEvents[event.Type]; !ok {
			typedEvents[event.Type] = event
		}
	}
	return builder
}

func (b *taskTraceBuilder) build(ctx context.Context, db *gorm.DB) (TaskTraceData, error) {
	b.addTaskCreated()
	b.addTaskQueued()
	b.addQueueAbort()
	b.addNodeSelected()
	b.addTaskStarted()
	b.addScoreSubmitted()
	b.addValidationRequest()
	b.addValidationResult()
	b.addResultUploadAvailable()
	b.addResultUploadStarted()
	b.addResultUploadCompleted()
	b.addAppResultAvailability()
	b.addTaskAborted()

	events := make([]TaskTraceEventData, 0, len(b.events))
	for _, event := range b.events {
		events = append(events, TaskTraceEventData{
			ID:               event.ID,
			Type:             event.Type,
			NodeAddress:      event.NodeAddress,
			TaskIDCommitment: event.TaskIDCommitment,
			CreatedAt:        event.CreatedAt.Unix(),
			Args:             event.Args,
		})
	}

	data := TaskTraceData{
		TaskIDCommitment:          b.task.TaskIDCommitment,
		TaskID:                    b.task.TaskID,
		Trace:                     b.trace,
		PrimaryTask:               buildTaskTraceTaskData(*b.task),
		ValidationGroupTasks:      buildTaskTraceGroupData(b.groupTasks, b.task.TaskIDCommitment),
		PersistedEvents:           events,
		MemoryTrace:               b.memoryTrace,
		AvailableFromExistingData: sortedSetValues(b.availableStored),
		AvailableFromMemory:       sortedSetValues(b.availableMemory),
		MissingData:               b.missingData,
	}
	if b.task.SelectedNode != "" {
		node, err := models.GetNodeByAddress(ctx, db, b.task.SelectedNode)
		if err == nil {
			snapshot, err := service.BuildTaskTraceNodeSnapshot(ctx, db, node)
			if err != nil {
				return TaskTraceData{}, err
			}
			data.CurrentSelectedNode = &snapshot
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return TaskTraceData{}, err
		}
	}
	return data, nil
}

func (b *taskTraceBuilder) addTaskCreated() {
	timestamp := unixFromNullTime(b.task.CreateTime)
	if timestamp != nil {
		b.markStored("task_created.timestamp")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "task_created",
		Timestamp: timestamp,
		Details: map[string]interface{}{
			"task_args":         b.task.TaskArgs,
			"task_type":         b.task.TaskType,
			"task_version":      b.task.TaskVersion,
			"timeout":           b.task.Timeout,
			"min_vram":          b.task.MinVRAM,
			"required_gpu":      b.task.RequiredGPU,
			"required_gpu_vram": b.task.RequiredGPUVRAM,
			"task_fee":          b.task.TaskFee.String(),
			"task_size":         b.task.TaskSize,
			"model_ids":         []string(b.task.ModelIDs),
			"creator":           b.task.Creator,
			"nonce":             b.task.Nonce,
			"sampling_seed":     b.task.SamplingSeed,
		},
	})
}

func (b *taskTraceBuilder) addTaskQueued() {
	timestamp := unixFromNullTime(b.task.CreateTime)
	details := map[string]interface{}{
		"status": b.task.Status,
	}
	if b.task.CreateTime.Valid {
		deadline := b.task.CreateTime.Time.Add(3*time.Minute + time.Duration(b.task.Timeout)*time.Second).Unix()
		details["queue_deadline"] = deadline
		b.markStored("task_queued.queue_deadline")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "task_queued",
		Timestamp: timestamp,
		Details:   details,
	})
}

func (b *taskTraceBuilder) addQueueAbort() {
	if b.task.Status != models.TaskEndAborted || b.task.SelectedNode != "" {
		return
	}
	event, ok := b.primaryEventByType("TaskEndAborted")
	if !ok {
		b.addMissing("queue_aborted", "timestamp", missingReasonNotPersisted)
		return
	}
	timestamp := event.CreatedAt.Unix()
	durations := b.durationFromCreate("queue_lifetime", event.CreatedAt)
	details := map[string]interface{}{
		"abort_reason": b.task.AbortReason,
	}
	if args, ok := parseAbortEvent(event); ok {
		details["abort_issuer"] = args.AbortIssuer
		details["last_status"] = args.LastStatus
	}
	b.markStored("queue_aborted.timestamp")
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "queue_aborted",
		Timestamp: &timestamp,
		Durations: durations,
		Details:   details,
	})
}

func (b *taskTraceBuilder) addNodeSelected() {
	var timestamp *int64
	details := map[string]interface{}{}
	if b.memoryTrace != nil && b.memoryTrace.NodeSelectedTime != nil {
		value := b.memoryTrace.NodeSelectedTime.Unix()
		timestamp = &value
		details["selected_node"] = b.memoryTrace.SelectedNode
		b.markMemory("node_selected.timestamp")
		if len(b.memoryTrace.NodeSelectionCandidatePool) > 0 || b.memoryTrace.NodeSelectionCandidatePoolTotalCount > 0 {
			details["candidate_pool"] = b.memoryTrace.NodeSelectionCandidatePool
			details["candidate_pool_total_count"] = b.memoryTrace.NodeSelectionCandidatePoolTotalCount
			details["candidate_pool_truncated"] = b.memoryTrace.NodeSelectionCandidatePoolTruncated
			b.markMemory("node_selected.candidate_pool")
			b.markMemory("node_selected.candidate_pool_total_count")
			b.markMemory("node_selected.candidate_pool_truncated")
		} else {
			b.addMissingMemory("node_selected", "candidate_pool")
		}
	} else {
		b.addMissingMemory("node_selected", "node_selected_time")
		b.addMissingMemory("node_selected", "selected_node")
		b.addMissingMemory("node_selected", "candidate_pool")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "node_selected",
		Timestamp: timestamp,
		Durations: b.durationBetween("queue_waiting_time", unixFromNullTime(b.task.CreateTime), timestamp),
		Details:   details,
	})
}

func (b *taskTraceBuilder) addTaskStarted() {
	timestamp := unixFromNullTime(b.task.StartTime)
	details := map[string]interface{}{
		"selected_node":   b.task.SelectedNode,
		"model_switched":  b.task.ModelSwtiched,
		"node_current":    "current_selected_node",
		"node_start_time": "memory_trace.start_node_snapshot",
	}
	if timestamp != nil {
		b.markStored("task_started.timestamp")
	}
	if b.memoryTrace != nil && b.memoryTrace.StartNodeSnapshot != nil {
		details["start_node_snapshot"] = b.memoryTrace.StartNodeSnapshot
		b.markMemory("task_started.start_node_snapshot")
	} else {
		b.addMissingMemory("task_started", "start_node_snapshot")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "task_started",
		Timestamp: timestamp,
		Durations: b.durationBetween("queue_waiting_time", unixFromNullTime(b.task.CreateTime), timestamp),
		Details:   details,
	})
}

func (b *taskTraceBuilder) addScoreSubmitted() {
	timestamp := unixFromNullTime(b.task.ScoreReadyTime)
	if timestamp != nil {
		b.markStored("score_submitted.timestamp")
	}
	name := "score_submitted"
	details := map[string]interface{}{"score": b.task.Score}
	if b.task.TaskError != models.TaskErrorNone {
		name = "task_error_reported"
		details = map[string]interface{}{"task_error": b.task.TaskError}
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      name,
		Timestamp: timestamp,
		Durations: b.durationBetween("execution_duration", unixFromNullTime(b.task.StartTime), timestamp),
		Details:   details,
	})
}

func (b *taskTraceBuilder) addValidationRequest() {
	var timestamp *int64
	details := map[string]interface{}{}
	if b.memoryTrace != nil && b.memoryTrace.ValidationRequestTime != nil {
		value := b.memoryTrace.ValidationRequestTime.Unix()
		timestamp = &value
		details["task_id"] = b.memoryTrace.ValidationTaskID
		details["task_id_commitments"] = b.memoryTrace.ValidationCommitments
		details["mode"] = b.memoryTrace.ValidationMode
		details["vrf_proof"] = b.memoryTrace.ValidationProofStoredAs
		details["public_key"] = b.memoryTrace.PublicKeyStoredAs
		b.markMemory("validation_request_and_group_reveal.timestamp")
		if b.memoryTrace.ValidationProofStoredAs == "not_stored" {
			b.addMissing("validation_request_and_group_reveal", "vrf_proof", missingReasonProofNotStored)
		}
		if b.memoryTrace.PublicKeyStoredAs == "not_stored" {
			b.addMissing("validation_request_and_group_reveal", "public_key", missingReasonProofNotStored)
		}
	} else {
		b.addMissingMemory("validation_request_and_group_reveal", "validation_request_time")
		b.addMissingMemory("validation_request_and_group_reveal", "task_id_commitments")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "validation_request_and_group_reveal",
		Timestamp: timestamp,
		Durations: b.durationBetween("score_to_validation_request", unixFromNullTime(b.task.ScoreReadyTime), timestamp),
		Details:   details,
	})
}

func (b *taskTraceBuilder) addValidationResult() {
	timestamp := unixFromNullTime(b.task.ValidatedTime)
	details := map[string]interface{}{}
	if timestamp != nil {
		b.markStored("validation_result.timestamp")
	}
	if b.memoryTrace != nil && b.memoryTrace.ValidationResult != nil {
		details["summary"] = b.memoryTrace.ValidationResult
		b.markMemory("validation_result.summary")
	} else {
		b.addMissingMemory("validation_result", "summary")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "validation_result",
		Timestamp: timestamp,
		Durations: b.durationBetween("validation_duration", b.memoryUnix("validation_request_time"), timestamp),
		Details:   details,
	})
}

func (b *taskTraceBuilder) addResultUploadAvailable() {
	var timestamp *int64
	details := map[string]interface{}{}
	if b.memoryTrace != nil && b.memoryTrace.ResultUploadAvailableTime != nil {
		value := b.memoryTrace.ResultUploadAvailableTime.Unix()
		timestamp = &value
		details["upload_eligibility_status"] = b.memoryTrace.UploadEligibilityStatus
		details["upload_category"] = b.memoryTrace.UploadCategory
		details["checkpoint_required"] = b.memoryTrace.CheckpointRequired
		details["slash_evidence_required"] = b.memoryTrace.SlashEvidenceRequired
		b.markMemory("result_upload_available.timestamp")
	} else {
		b.addMissingMemory("result_upload_available", "relay_result_upload_available_time")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "result_upload_available",
		Timestamp: timestamp,
		Details:   details,
	})
}

func (b *taskTraceBuilder) addResultUploadStarted() {
	var timestamp *int64
	details := map[string]interface{}{}
	if b.memoryTrace != nil && b.memoryTrace.ResultUploadStartedTime != nil {
		value := b.memoryTrace.ResultUploadStartedTime.Unix()
		timestamp = &value
		details["file_count"] = b.memoryTrace.ResultUploadFileCount
		details["files"] = b.memoryTrace.ResultUploadFiles
		details["checkpoint_present"] = b.memoryTrace.CheckpointPresent
		b.markMemory("result_upload_started.timestamp")
	} else {
		b.addMissingMemory("result_upload_started", "result_upload_started_time")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "result_upload_started",
		Timestamp: timestamp,
		Details:   details,
	})
}

func (b *taskTraceBuilder) addResultUploadCompleted() {
	timestamp := unixFromNullTime(b.task.ResultUploadedTime)
	if timestamp != nil {
		b.markStored("result_upload_completed.timestamp")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "result_upload_completed",
		Timestamp: timestamp,
		Durations: b.durationBetween("upload_result_duration", b.memoryUnix("result_upload_started_time"), timestamp),
		Details: map[string]interface{}{
			"status": b.task.Status,
		},
	})
}

func (b *taskTraceBuilder) addAppResultAvailability() {
	timestamp := unixFromNullTime(b.task.ResultUploadedTime)
	details := map[string]interface{}{
		"relay_result_available_time": timestamp,
	}
	var appFetchTimestamp *int64
	if b.memoryTrace != nil && b.memoryTrace.FirstAppResultFetchedTime != nil {
		value := b.memoryTrace.FirstAppResultFetchedTime.Unix()
		appFetchTimestamp = &value
		details["app_result_fetched_time"] = value
		details["latest_app_result_fetched_time"] = b.memoryTrace.LatestAppResultFetchedTime.Unix()
		details["fetches"] = b.memoryTrace.AppResultFetches
		b.markMemory("app_result_availability.app_result_fetched_time")
	} else {
		b.addMissingMemory("app_result_availability", "app_result_fetched_time")
	}
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "app_result_availability",
		Timestamp: timestamp,
		Durations: b.durationBetween("upload_to_app_fetch", timestamp, appFetchTimestamp),
		Details:   details,
	})
}

func (b *taskTraceBuilder) addTaskAborted() {
	if b.task.Status != models.TaskEndAborted {
		return
	}
	event, ok := b.primaryEventByType("TaskEndAborted")
	if !ok {
		b.addMissing("task_aborted", "timestamp", missingReasonNotPersisted)
		return
	}
	timestamp := event.CreatedAt.Unix()
	durations := b.durationFromCreate("total_task_lifetime", event.CreatedAt)
	if previous := b.previousTraceTimestamp(); previous != nil {
		durations = append(durations, TaskTraceDuration{
			Name:    "previous_step_to_abort",
			Seconds: timestamp - *previous,
		})
	}
	details := map[string]interface{}{
		"abort_reason":  b.task.AbortReason,
		"selected_node": b.task.SelectedNode,
	}
	if args, ok := parseAbortEvent(event); ok {
		details["abort_issuer"] = args.AbortIssuer
		details["last_status"] = args.LastStatus
	}
	b.markStored("task_aborted.timestamp")
	b.trace = append(b.trace, TaskTraceStep{
		Name:      "task_aborted",
		Timestamp: &timestamp,
		Durations: durations,
		Details:   details,
	})
}

func (b *taskTraceBuilder) markStored(field string) {
	b.availableStored[field] = struct{}{}
}

func (b *taskTraceBuilder) primaryEventByType(eventType string) (models.Event, bool) {
	typedEvents, ok := b.eventByCommitment[b.task.TaskIDCommitment]
	if !ok {
		return models.Event{}, false
	}
	event, ok := typedEvents[eventType]
	return event, ok
}

func (b *taskTraceBuilder) markMemory(field string) {
	b.availableMemory[field] = struct{}{}
}

func (b *taskTraceBuilder) addMissingMemory(step, field string) {
	reason := missingReasonExpired
	if !b.tracingEnabled {
		reason = missingReasonDisabled
	} else if b.memoryAvailable {
		reason = missingReasonNotReached
	}
	b.addMissing(step, field, reason)
}

func (b *taskTraceBuilder) addMissing(step, field, reason string) {
	b.missingData = append(b.missingData, TaskTraceMissingData{
		Step:   step,
		Field:  field,
		Reason: reason,
	})
}

func (b *taskTraceBuilder) memoryUnix(field string) *int64 {
	if b.memoryTrace == nil {
		return nil
	}
	var t *time.Time
	switch field {
	case "validation_request_time":
		t = b.memoryTrace.ValidationRequestTime
	case "result_upload_started_time":
		t = b.memoryTrace.ResultUploadStartedTime
	}
	if t == nil {
		return nil
	}
	value := t.Unix()
	return &value
}

func (b *taskTraceBuilder) durationBetween(name string, start, end *int64) []TaskTraceDuration {
	if start == nil || end == nil {
		return nil
	}
	return []TaskTraceDuration{{Name: name, Seconds: *end - *start}}
}

func (b *taskTraceBuilder) durationFromCreate(name string, end time.Time) []TaskTraceDuration {
	if !b.task.CreateTime.Valid {
		return nil
	}
	return []TaskTraceDuration{{Name: name, Seconds: int64(end.Sub(b.task.CreateTime.Time).Seconds())}}
}

func (b *taskTraceBuilder) previousTraceTimestamp() *int64 {
	for i := len(b.trace) - 1; i >= 0; i-- {
		if b.trace[i].Timestamp != nil {
			return b.trace[i].Timestamp
		}
	}
	return nil
}

func getTaskTraceGroupTasks(ctx context.Context, db *gorm.DB, task *models.InferenceTask) ([]models.InferenceTask, error) {
	if task.TaskID == "" {
		return []models.InferenceTask{*task}, nil
	}
	tasks, err := models.GetTaskGroupByTaskID(ctx, db, task.TaskID)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func queryTaskTraceEvents(ctx context.Context, db *gorm.DB, tasks []models.InferenceTask) ([]models.Event, error) {
	commitments := make([]string, 0, len(tasks))
	for _, task := range tasks {
		commitments = append(commitments, task.TaskIDCommitment)
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var events []models.Event
	if err := db.WithContext(dbCtx).
		Where("task_id_commitment IN ?", commitments).
		Order("created_at ASC, id ASC").
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func buildTaskTraceTaskData(task models.InferenceTask) TaskTraceTaskData {
	var qosScore *int64
	if task.QOSScore.Valid {
		value := task.QOSScore.Int64
		qosScore = &value
	}
	return TaskTraceTaskData{
		TaskIDCommitment:   task.TaskIDCommitment,
		TaskID:             task.TaskID,
		Status:             task.Status,
		SelectedNode:       task.SelectedNode,
		Score:              task.Score,
		TaskError:          task.TaskError,
		QOSScore:           qosScore,
		AbortReason:        task.AbortReason,
		CreateTime:         unixFromNullTime(task.CreateTime),
		StartTime:          unixFromNullTime(task.StartTime),
		ScoreReadyTime:     unixFromNullTime(task.ScoreReadyTime),
		ValidatedTime:      unixFromNullTime(task.ValidatedTime),
		ResultUploadedTime: unixFromNullTime(task.ResultUploadedTime),
	}
}

func buildTaskTraceGroupData(tasks []models.InferenceTask, primaryCommitment string) []TaskTraceTaskData {
	results := make([]TaskTraceTaskData, 0, len(tasks))
	for _, task := range tasks {
		if task.TaskIDCommitment == primaryCommitment {
			continue
		}
		results = append(results, buildTaskTraceTaskData(task))
	}
	return results
}

func unixFromNullTime(value sql.NullTime) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Time.Unix()
	return &result
}

func sortedSetValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func parseAbortEvent(event models.Event) (models.TaskEndAbortedEvent, bool) {
	var args models.TaskEndAbortedEvent
	if err := json.Unmarshal([]byte(event.Args), &args); err != nil {
		return models.TaskEndAbortedEvent{}, false
	}
	return args, true
}
