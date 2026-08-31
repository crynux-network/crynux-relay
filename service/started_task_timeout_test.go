package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetTimedOutRunningTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	now := time.Now()
	tasks := []models.InferenceTask{
		{
			TaskIDCommitment: "expired-started",
			Status:           models.TaskStarted,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "active-started",
			Status:           models.TaskStarted,
			StartTime:        sql.NullTime{Time: now.Add(-30 * time.Second), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-parameters-uploaded",
			Status:           models.TaskParametersUploaded,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-queued",
			Status:           models.TaskQueued,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-score-ready",
			TaskType:         models.TaskTypeSDFTLora,
			Status:           models.TaskScoreReady,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-error-reported",
			TaskType:         models.TaskTypeSDFTLora,
			Status:           models.TaskErrorReported,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-validated",
			TaskType:         models.TaskTypeSDFTLora,
			Status:           models.TaskValidated,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-group-validated",
			TaskType:         models.TaskTypeSDFTLora,
			Status:           models.TaskGroupValidated,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-aborted",
			Status:           models.TaskEndAborted,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
	}
	for i := range tasks {
		if err := db.Create(&tasks[i]).Error; err != nil {
			t.Fatalf("failed to seed task %s: %v", tasks[i].TaskIDCommitment, err)
		}
	}

	timedOutTasks, err := getTimedOutRunningTasks(context.Background(), db, now)
	if err != nil {
		t.Fatalf("failed to get timed out running tasks: %v", err)
	}

	got := make(map[string]struct{}, len(timedOutTasks))
	for _, task := range timedOutTasks {
		got[task.TaskIDCommitment] = struct{}{}
	}
	for _, taskIDCommitment := range []string{
		"expired-started",
		"expired-parameters-uploaded",
		"expired-score-ready",
		"expired-error-reported",
		"expired-validated",
		"expired-group-validated",
	} {
		if _, ok := got[taskIDCommitment]; !ok {
			t.Fatalf("expected %s to be timed out, got %#v", taskIDCommitment, got)
		}
	}
	if len(got) != 6 {
		t.Fatalf("expected only six timed out running tasks, got %#v", got)
	}
}

func TestGetTimedOutQueuedTasks(t *testing.T) {
	initServiceTestConfig(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	now := time.Now()
	queueTimeout := time.Duration(config.GetConfig().TaskPricing.QueueTimeoutSeconds) * time.Second
	tasks := []models.InferenceTask{
		{
			TaskIDCommitment: "expired-sdft-queued",
			TaskType:         models.TaskTypeSDFTLora,
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-5 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "active-sdft-queued",
			TaskType:         models.TaskTypeSDFTLora,
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-30 * time.Second), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-relay-owned-queued",
			TaskType:         models.TaskTypeSD,
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-queueTimeout - time.Minute), Valid: true},
		},
		{
			TaskIDCommitment: "active-relay-owned-queued",
			TaskType:         models.TaskTypeSD,
			Status:           models.TaskQueued,
			// Older than the SDFT 3-minute earliest cutoff, but still inside the
			// relay-owned queue timeout. Must not be selected.
			CreateTime: sql.NullTime{Time: now.Add(-5 * time.Minute), Valid: true},
		},
		{
			TaskIDCommitment: "expired-started",
			Status:           models.TaskStarted,
			CreateTime:       sql.NullTime{Time: now.Add(-5 * time.Minute), Valid: true},
			StartTime:        sql.NullTime{Time: now.Add(-5 * time.Minute), Valid: true},
			Timeout:          60,
		},
	}
	for i := range tasks {
		if err := db.Create(&tasks[i]).Error; err != nil {
			t.Fatalf("failed to seed task %s: %v", tasks[i].TaskIDCommitment, err)
		}
	}

	timedOutTasks, err := getTimedOutQueuedTasks(context.Background(), db, now)
	if err != nil {
		t.Fatalf("failed to get timed out queued tasks: %v", err)
	}

	got := make(map[string]struct{}, len(timedOutTasks))
	for _, task := range timedOutTasks {
		got[task.TaskIDCommitment] = struct{}{}
	}
	for _, taskIDCommitment := range []string{"expired-sdft-queued", "expired-relay-owned-queued"} {
		if _, ok := got[taskIDCommitment]; !ok {
			t.Fatalf("expected %s to be timed out, got %#v", taskIDCommitment, got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected only two timed out queued tasks, got %#v", got)
	}
}

func TestGetQueueDeadline(t *testing.T) {
	initServiceTestConfig(t)
	now := time.Now().Truncate(time.Second)
	tests := []struct {
		name string
		task models.InferenceTask
		want time.Time
	}{
		{
			name: "relay owned sd",
			task: models.InferenceTask{
				TaskType:   models.TaskTypeSD,
				Status:     models.TaskStarted,
				CreateTime: sql.NullTime{Time: now, Valid: true},
				Timeout:    60,
			},
			want: now.Add(21600 * time.Second),
		},
		{
			name: "sdft",
			task: models.InferenceTask{
				TaskType:   models.TaskTypeSDFTLora,
				Status:     models.TaskQueued,
				CreateTime: sql.NullTime{Time: now, Valid: true},
				Timeout:    120,
			},
			want: now.Add(3*time.Minute + 120*time.Second),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deadline, ok := GetQueueDeadline(&test.task)
			if !ok {
				t.Fatal("expected queue deadline")
			}
			if !deadline.Equal(test.want) {
				t.Fatalf("queue deadline = %s, want %s", deadline, test.want)
			}
		})
	}
}

func TestGetTaskDeadlineRelayOwnedPhases(t *testing.T) {
	initServiceTestConfig(t)
	now := time.Now().Truncate(time.Second)
	tests := []struct {
		name       string
		task       models.InferenceTask
		wantPhase  TaskTimeoutPhase
		wantWaiter string
		wantReason models.TaskAbortReason
		want       time.Time
	}{
		{
			name: "queue",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskQueued,
				CreateTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseQueue, wantWaiter: "relay", wantReason: models.TaskAbortTimeout,
			want: now.Add(21600 * time.Second),
		},
		{
			name: "execution",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskStarted, Timeout: 123,
				StartTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseExecution, wantWaiter: "node", wantReason: models.TaskAbortTimeout,
			want: now.Add(123 * time.Second),
		},
		{
			name: "app validation",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskScoreReady,
				ScoreReadyTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseAppValidation, wantWaiter: "app", wantReason: models.TaskAbortCreatorValidationTimeout,
			want: now.Add(600 * time.Second),
		},
		{
			name: "result upload",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskValidated,
				ValidatedTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseResultUpload, wantWaiter: "node", wantReason: models.TaskAbortResultUploadTimeout,
			want: now.Add(600 * time.Second),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deadline, phase, waiter, reason, ok := GetTaskDeadline(&test.task)
			if !ok {
				t.Fatal("expected deadline")
			}
			if !deadline.Equal(test.want) || phase != test.wantPhase || waiter != test.wantWaiter || reason != test.wantReason {
				t.Fatalf("got deadline=%s phase=%s waiter=%s reason=%d", deadline, phase, waiter, reason)
			}
		})
	}
}

func TestGetTaskTransitionDeadlinePreservesSDFTTimeout(t *testing.T) {
	initServiceTestConfig(t)
	startTime := time.Now().Truncate(time.Second)
	task := models.InferenceTask{
		TaskType:  models.TaskTypeSDFTLora,
		StartTime: sql.NullTime{Time: startTime, Valid: true},
		Timeout:   120,
	}
	want := startTime.Add(120 * time.Second)
	for _, status := range []models.TaskStatus{
		models.TaskScoreReady,
		models.TaskErrorReported,
		models.TaskValidated,
		models.TaskGroupValidated,
	} {
		deadline, err := getTaskTransitionDeadline(&task, status, startTime.Add(time.Minute))
		if err != nil {
			t.Fatalf("status %d: %v", status, err)
		}
		if !deadline.Valid || !deadline.Time.Equal(want) {
			t.Fatalf("status %d deadline = %v, want %v", status, deadline, want)
		}
	}
}

func TestGetTaskTransitionDeadlineUsesRelayOwnedPhases(t *testing.T) {
	initServiceTestConfig(t)
	transitionTime := time.Now().Truncate(time.Second)
	task := models.InferenceTask{
		TaskType: models.TaskTypeSD,
	}
	tests := []struct {
		status  models.TaskStatus
		seconds uint64
	}{
		{status: models.TaskScoreReady, seconds: config.GetConfig().TaskPricing.AppValidationTimeoutSeconds},
		{status: models.TaskErrorReported, seconds: config.GetConfig().TaskPricing.AppValidationTimeoutSeconds},
		{status: models.TaskValidated, seconds: config.GetConfig().TaskPricing.ResultUploadTimeoutSeconds},
		{status: models.TaskGroupValidated, seconds: config.GetConfig().TaskPricing.ResultUploadTimeoutSeconds},
	}
	for _, test := range tests {
		deadline, err := getTaskTransitionDeadline(&task, test.status, transitionTime)
		if err != nil {
			t.Fatalf("status %d: %v", test.status, err)
		}
		want := transitionTime.Add(time.Duration(test.seconds) * time.Second)
		if !deadline.Valid || !deadline.Time.Equal(want) {
			t.Fatalf("status %d deadline = %v, want %v", test.status, deadline, want)
		}
	}
}

func TestShouldUpdateNodeQosScoreOnAbort(t *testing.T) {
	tests := []struct {
		name        string
		task        models.InferenceTask
		wantUpdated bool
	}{
		{
			name: "group validation result",
			task: models.InferenceTask{
				QOSScore:    sql.NullInt64{Int64: 5, Valid: true},
				AbortReason: models.TaskAbortIncorrectResult,
			},
			wantUpdated: true,
		},
		{
			name: "result upload timeout",
			task: models.InferenceTask{
				QOSScore:    sql.NullInt64{Int64: 5, Valid: true},
				AbortReason: models.TaskAbortResultUploadTimeout,
			},
			wantUpdated: false,
		},
		{
			name: "creator validation timeout",
			task: models.InferenceTask{
				QOSScore:    sql.NullInt64{Int64: 5, Valid: true},
				AbortReason: models.TaskAbortCreatorValidationTimeout,
			},
			wantUpdated: false,
		},
		{
			name: "no validation score",
			task: models.InferenceTask{
				AbortReason: models.TaskAbortTimeout,
			},
			wantUpdated: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldUpdateNodeQosScoreOnAbort(&test.task); got != test.wantUpdated {
				t.Fatalf("shouldUpdateNodeQosScoreOnAbort() = %t, want %t", got, test.wantUpdated)
			}
		})
	}
}

func TestUsesRelayOwnedTimeoutsByTaskType(t *testing.T) {
	if !UsesRelayOwnedTimeouts(&models.InferenceTask{TaskType: models.TaskTypeSD}) {
		t.Fatal("SD must use relay-owned timeouts")
	}
	if !UsesRelayOwnedTimeouts(&models.InferenceTask{TaskType: models.TaskTypeLLM}) {
		t.Fatal("LLM must use relay-owned timeouts")
	}
	if UsesRelayOwnedTimeouts(&models.InferenceTask{TaskType: models.TaskTypeSDFTLora, Timeout: 60}) {
		t.Fatal("SDFT must keep creator-supplied timeouts")
	}
}
