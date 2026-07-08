package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"sync"
	"time"
)

const (
	taskTraceCleanupInterval    = time.Hour
	taskTraceCandidatePoolLimit = 200
)

type TaskTraceNodeSelectionCandidate struct {
	Address      string  `json:"address"`
	CardName     string  `json:"card_name"`
	StakingScore float64 `json:"staking_score"`
	QOSScore     float64 `json:"qos_score"`
	ProbWeight   float64 `json:"prob_weight"`
}

type TaskTraceValidationTask struct {
	TaskIDCommitment string                 `json:"task_id_commitment"`
	Status           models.TaskStatus      `json:"status"`
	SelectedNode     string                 `json:"selected_node"`
	Score            string                 `json:"score,omitempty"`
	TaskError        models.TaskError       `json:"task_error"`
	QOSScore         *int64                 `json:"qos_score,omitempty"`
	AbortReason      models.TaskAbortReason `json:"abort_reason"`
	ValidatedTime    *time.Time             `json:"validated_time,omitempty"`
	ScoreReadyTime   *time.Time             `json:"score_ready_time,omitempty"`
}

type TaskTraceValidationResult struct {
	PassedCount  int                       `json:"passed_count"`
	RefundCount  int                       `json:"refund_count"`
	InvalidCount int                       `json:"invalid_count"`
	AbortedCount int                       `json:"aborted_count"`
	Tasks        []TaskTraceValidationTask `json:"tasks"`
}

type TaskTraceUploadFile struct {
	Index string `json:"index"`
	Type  string `json:"type"`
}

type TaskTraceResultFetch struct {
	Time        time.Time `json:"time"`
	Kind        string    `json:"kind"`
	ResultIndex string    `json:"result_index,omitempty"`
}

type TaskTraceRecord struct {
	TaskIDCommitment string    `json:"task_id_commitment"`
	TaskID           string    `json:"task_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	ExpiresAt        time.Time `json:"expires_at"`

	SelectedNode                         string                            `json:"selected_node,omitempty"`
	NodeSelectedTime                     *time.Time                        `json:"node_selected_time,omitempty"`
	NodeSelectionCandidatePool           []TaskTraceNodeSelectionCandidate `json:"node_selection_candidate_pool,omitempty"`
	NodeSelectionCandidatePoolTotalCount int                               `json:"node_selection_candidate_pool_total_count,omitempty"`
	NodeSelectionCandidatePoolTruncated  bool                              `json:"node_selection_candidate_pool_truncated,omitempty"`

	StartNodeSnapshot *models.SlashEvidenceNodeSnapshot `json:"start_node_snapshot,omitempty"`

	ValidationRequestTime   *time.Time `json:"validation_request_time,omitempty"`
	ValidationTaskID        string     `json:"validation_task_id,omitempty"`
	ValidationCommitments   []string   `json:"validation_task_id_commitments,omitempty"`
	ValidationMode          string     `json:"validation_mode,omitempty"`
	ValidationProofStoredAs string     `json:"validation_proof_stored_as,omitempty"`
	PublicKeyStoredAs       string     `json:"public_key_stored_as,omitempty"`

	ValidationResultTime *time.Time                 `json:"validation_result_time,omitempty"`
	ValidationResult     *TaskTraceValidationResult `json:"validation_result,omitempty"`

	ResultUploadAvailableTime *time.Time `json:"relay_result_upload_available_time,omitempty"`
	UploadEligibilityStatus   string     `json:"upload_eligibility_status,omitempty"`
	UploadCategory            string     `json:"upload_category,omitempty"`
	CheckpointRequired        bool       `json:"checkpoint_required"`
	SlashEvidenceRequired     bool       `json:"slash_evidence_required"`

	ResultUploadStartedTime *time.Time            `json:"result_upload_started_time,omitempty"`
	ResultUploadFileCount   int                   `json:"result_upload_file_count,omitempty"`
	ResultUploadFiles       []TaskTraceUploadFile `json:"result_upload_files,omitempty"`
	CheckpointPresent       bool                  `json:"checkpoint_present"`

	FirstAppResultFetchedTime  *time.Time             `json:"first_app_result_fetched_time,omitempty"`
	LatestAppResultFetchedTime *time.Time             `json:"latest_app_result_fetched_time,omitempty"`
	AppResultFetches           []TaskTraceResultFetch `json:"app_result_fetches,omitempty"`
}

type TaskTraceStore struct {
	mu                sync.RWMutex
	records           map[string]*TaskTraceRecord
	taskIDCommitments map[string]map[string]struct{}
}

var defaultTaskTraceStore = NewTaskTraceStore()

func NewTaskTraceStore() *TaskTraceStore {
	return &TaskTraceStore{
		records:           make(map[string]*TaskTraceRecord),
		taskIDCommitments: make(map[string]map[string]struct{}),
	}
}

func GetTaskTraceStore() *TaskTraceStore {
	return defaultTaskTraceStore
}

func (s *TaskTraceStore) Enabled() bool {
	cfg := config.GetConfig()
	return cfg != nil && cfg.Task.TaskTracingDurationDays > 0
}

func (s *TaskTraceStore) retentionDuration() time.Duration {
	cfg := config.GetConfig()
	if cfg == nil || cfg.Task.TaskTracingDurationDays == 0 {
		return 0
	}
	return time.Duration(cfg.Task.TaskTracingDurationDays) * 24 * time.Hour
}

func (s *TaskTraceStore) upsert(taskIDCommitment string, now time.Time, fn func(record *TaskTraceRecord)) {
	retention := s.retentionDuration()
	if taskIDCommitment == "" || retention == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[taskIDCommitment]
	if !ok {
		record = &TaskTraceRecord{
			TaskIDCommitment: taskIDCommitment,
			CreatedAt:        now,
		}
		s.records[taskIDCommitment] = record
	}
	oldTaskID := record.TaskID
	fn(record)
	record.UpdatedAt = now
	record.ExpiresAt = now.Add(retention)
	if oldTaskID != record.TaskID {
		s.removeGroupIndexLocked(oldTaskID, taskIDCommitment)
		s.addGroupIndexLocked(record.TaskID, taskIDCommitment)
	}
}

func (s *TaskTraceStore) RecordNodeSelected(taskIDCommitment, selectedNode string, selectedTime time.Time, candidatePool []TaskTraceNodeSelectionCandidate, candidatePoolTotalCount int, candidatePoolTruncated bool) {
	s.upsert(taskIDCommitment, time.Now().UTC(), func(record *TaskTraceRecord) {
		t := selectedTime.UTC()
		record.SelectedNode = selectedNode
		record.NodeSelectedTime = &t
		record.NodeSelectionCandidatePool, record.NodeSelectionCandidatePoolTotalCount, record.NodeSelectionCandidatePoolTruncated = normalizeTaskTraceCandidatePool(candidatePool, candidatePoolTotalCount, candidatePoolTruncated)
	})
}

func (s *TaskTraceStore) RecordTaskStarted(task *models.InferenceTask, snapshot models.SlashEvidenceNodeSnapshot) {
	s.upsert(task.TaskIDCommitment, time.Now().UTC(), func(record *TaskTraceRecord) {
		record.SelectedNode = task.SelectedNode
		record.StartNodeSnapshot = &snapshot
		if task.TaskID != "" {
			record.TaskID = task.TaskID
		}
	})
}

func (s *TaskTraceStore) RecordValidationRequest(taskID string, commitments []string, mode string) {
	now := time.Now().UTC()
	for _, taskIDCommitment := range commitments {
		commitmentsCopy := append([]string(nil), commitments...)
		s.upsert(taskIDCommitment, now, func(record *TaskTraceRecord) {
			record.TaskID = taskID
			record.ValidationRequestTime = &now
			record.ValidationTaskID = taskID
			record.ValidationCommitments = commitmentsCopy
			record.ValidationMode = mode
			record.ValidationProofStoredAs = "not_stored"
			record.PublicKeyStoredAs = "not_stored"
		})
	}
}

func (s *TaskTraceStore) RecordValidationResult(tasks []*models.InferenceTask) {
	if len(tasks) == 0 {
		return
	}
	now := time.Now().UTC()
	summary := buildTaskTraceValidationResult(tasks)
	for _, task := range tasks {
		s.upsert(task.TaskIDCommitment, now, func(record *TaskTraceRecord) {
			record.TaskID = task.TaskID
			record.ValidationResultTime = &now
			record.ValidationResult = summary
			record.ResultUploadAvailableTime = &now
			record.UploadEligibilityStatus = stringForTaskStatus(task.Status)
			record.UploadCategory = uploadCategoryForTask(task)
			record.CheckpointRequired = task.TaskType == models.TaskTypeSDFTLora
			record.SlashEvidenceRequired = task.Status == models.TaskEndInvalidated
		})
	}
}

func (s *TaskTraceStore) RecordResultUploadStarted(task *models.InferenceTask, files []TaskTraceUploadFile, checkpointPresent bool) {
	now := time.Now().UTC()
	s.upsert(task.TaskIDCommitment, now, func(record *TaskTraceRecord) {
		record.TaskID = task.TaskID
		record.ResultUploadStartedTime = &now
		record.ResultUploadFileCount = len(files)
		record.ResultUploadFiles = append([]TaskTraceUploadFile(nil), files...)
		record.CheckpointPresent = checkpointPresent
	})
}

func (s *TaskTraceStore) RecordAppResultFetched(taskIDCommitment, kind, resultIndex string) {
	now := time.Now().UTC()
	s.upsert(taskIDCommitment, now, func(record *TaskTraceRecord) {
		record.LatestAppResultFetchedTime = &now
		if record.FirstAppResultFetchedTime == nil {
			record.FirstAppResultFetchedTime = &now
		}
		record.AppResultFetches = append(record.AppResultFetches, TaskTraceResultFetch{
			Time:        now,
			Kind:        kind,
			ResultIndex: resultIndex,
		})
	})
}

func (s *TaskTraceStore) Get(taskIDCommitment string, now time.Time) (*TaskTraceRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[taskIDCommitment]
	if !ok {
		return nil, false
	}
	if !record.ExpiresAt.After(now) {
		s.deleteLocked(taskIDCommitment)
		return nil, false
	}
	return cloneTaskTraceRecord(record), true
}

func (s *TaskTraceStore) GetByTaskID(taskID string, now time.Time) []TaskTraceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	commitments := s.taskIDCommitments[taskID]
	records := make([]TaskTraceRecord, 0, len(commitments))
	for taskIDCommitment := range commitments {
		record, ok := s.records[taskIDCommitment]
		if !ok {
			continue
		}
		if !record.ExpiresAt.After(now) {
			s.deleteLocked(taskIDCommitment)
			continue
		}
		records = append(records, *cloneTaskTraceRecord(record))
	}
	return records
}

func (s *TaskTraceStore) CleanupExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for taskIDCommitment, record := range s.records {
		if !record.ExpiresAt.After(now) {
			s.deleteLocked(taskIDCommitment)
		}
	}
}

func (s *TaskTraceStore) StartCleanup(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	timer := time.NewTicker(taskTraceCleanupInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.CleanupExpired(time.Now().UTC())
		}
	}
}

func (s *TaskTraceStore) addGroupIndexLocked(taskID, taskIDCommitment string) {
	if taskID == "" {
		return
	}
	commitments, ok := s.taskIDCommitments[taskID]
	if !ok {
		commitments = make(map[string]struct{})
		s.taskIDCommitments[taskID] = commitments
	}
	commitments[taskIDCommitment] = struct{}{}
}

func (s *TaskTraceStore) removeGroupIndexLocked(taskID, taskIDCommitment string) {
	if taskID == "" {
		return
	}
	commitments, ok := s.taskIDCommitments[taskID]
	if !ok {
		return
	}
	delete(commitments, taskIDCommitment)
	if len(commitments) == 0 {
		delete(s.taskIDCommitments, taskID)
	}
}

func (s *TaskTraceStore) deleteLocked(taskIDCommitment string) {
	record, ok := s.records[taskIDCommitment]
	if ok {
		s.removeGroupIndexLocked(record.TaskID, taskIDCommitment)
	}
	delete(s.records, taskIDCommitment)
}

func cloneTaskTraceRecord(record *TaskTraceRecord) *TaskTraceRecord {
	clone := *record
	clone.NodeSelectionCandidatePool = append([]TaskTraceNodeSelectionCandidate(nil), record.NodeSelectionCandidatePool...)
	clone.ValidationCommitments = append([]string(nil), record.ValidationCommitments...)
	clone.ResultUploadFiles = append([]TaskTraceUploadFile(nil), record.ResultUploadFiles...)
	clone.AppResultFetches = append([]TaskTraceResultFetch(nil), record.AppResultFetches...)
	if record.StartNodeSnapshot != nil {
		snapshot := *record.StartNodeSnapshot
		snapshot.Models = append([]models.SlashEvidenceNodeModel(nil), record.StartNodeSnapshot.Models...)
		clone.StartNodeSnapshot = &snapshot
	}
	if record.ValidationResult != nil {
		result := *record.ValidationResult
		result.Tasks = append([]TaskTraceValidationTask(nil), record.ValidationResult.Tasks...)
		clone.ValidationResult = &result
	}
	return &clone
}

func normalizeTaskTraceCandidatePool(candidatePool []TaskTraceNodeSelectionCandidate, totalCount int, truncated bool) ([]TaskTraceNodeSelectionCandidate, int, bool) {
	if totalCount < len(candidatePool) {
		totalCount = len(candidatePool)
	}
	if len(candidatePool) > taskTraceCandidatePoolLimit {
		candidatePool = candidatePool[:taskTraceCandidatePoolLimit]
		truncated = true
	}
	if totalCount > len(candidatePool) {
		truncated = true
	}
	return append([]TaskTraceNodeSelectionCandidate(nil), candidatePool...), totalCount, truncated
}

func buildTaskTraceValidationResult(tasks []*models.InferenceTask) *TaskTraceValidationResult {
	result := &TaskTraceValidationResult{
		Tasks: make([]TaskTraceValidationTask, 0, len(tasks)),
	}
	for _, task := range tasks {
		switch task.Status {
		case models.TaskValidated, models.TaskGroupValidated, models.TaskEndSuccess, models.TaskEndGroupSuccess:
			result.PassedCount++
		case models.TaskEndGroupRefund:
			result.RefundCount++
		case models.TaskEndInvalidated:
			result.InvalidCount++
		case models.TaskEndAborted:
			result.AbortedCount++
		}
		var qosScore *int64
		if task.QOSScore.Valid {
			value := task.QOSScore.Int64
			qosScore = &value
		}
		result.Tasks = append(result.Tasks, TaskTraceValidationTask{
			TaskIDCommitment: task.TaskIDCommitment,
			Status:           task.Status,
			SelectedNode:     task.SelectedNode,
			Score:            task.Score,
			TaskError:        task.TaskError,
			QOSScore:         qosScore,
			AbortReason:      task.AbortReason,
			ValidatedTime:    nullTimePtr(task.ValidatedTime),
			ScoreReadyTime:   nullTimePtr(task.ScoreReadyTime),
		})
	}
	return result
}

func stringForTaskStatus(status models.TaskStatus) string {
	switch status {
	case models.TaskQueued:
		return "task_queued"
	case models.TaskStarted:
		return "task_started"
	case models.TaskParametersUploaded:
		return "task_parameters_uploaded"
	case models.TaskErrorReported:
		return "task_error_reported"
	case models.TaskScoreReady:
		return "task_score_ready"
	case models.TaskValidated:
		return "task_validated"
	case models.TaskGroupValidated:
		return "task_group_validated"
	case models.TaskEndInvalidated:
		return "task_end_invalidated"
	case models.TaskEndSuccess:
		return "task_end_success"
	case models.TaskEndAborted:
		return "task_end_aborted"
	case models.TaskEndGroupRefund:
		return "task_end_group_refund"
	case models.TaskEndGroupSuccess:
		return "task_end_group_success"
	default:
		return "unknown"
	}
}

func uploadCategoryForTask(task *models.InferenceTask) string {
	switch task.Status {
	case models.TaskValidated, models.TaskGroupValidated:
		return "successful_task"
	case models.TaskEndInvalidated:
		return "invalidated_task_with_slash_evidence"
	case models.TaskEndGroupRefund:
		return "refund_task"
	default:
		return "not_uploadable"
	}
}
