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
			Status:           models.TaskScoreReady,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-error-reported",
			Status:           models.TaskErrorReported,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-validated",
			Status:           models.TaskValidated,
			StartTime:        sql.NullTime{Time: now.Add(-2 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-group-validated",
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
	sdUnits := uint64(512 * 512)
	queueTimeout := time.Duration(config.GetConfig().TaskPricing.QueueTimeoutSeconds) * time.Second
	tasks := []models.InferenceTask{
		{
			TaskIDCommitment: "expired-legacy-queued",
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-5 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "active-legacy-queued",
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-30 * time.Second), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "expired-relay-owned-queued",
			TaskType:         models.TaskTypeSD,
			SDUnits:          &sdUnits,
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-queueTimeout - time.Minute), Valid: true},
		},
		{
			TaskIDCommitment: "active-relay-owned-queued",
			TaskType:         models.TaskTypeSD,
			SDUnits:          &sdUnits,
			Status:           models.TaskQueued,
			// Older than the legacy 3-minute earliest cutoff, but still inside
			// the relay-owned queue timeout. Must not be selected.
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
	for _, taskIDCommitment := range []string{"expired-legacy-queued", "expired-relay-owned-queued"} {
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
	sdUnits := uint64(512 * 512)
	tests := []struct {
		name string
		task models.InferenceTask
		want time.Time
	}{
		{
			name: "relay owned sd",
			task: models.InferenceTask{
				TaskType:   models.TaskTypeSD,
				SDUnits:    &sdUnits,
				Status:     models.TaskStarted,
				CreateTime: sql.NullTime{Time: now, Valid: true},
				Timeout:    60,
			},
			want: now.Add(21600 * time.Second),
		},
		{
			name: "sdft legacy",
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
	sdUnits := uint64(512 * 512)
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
				TaskType: models.TaskTypeSD, Status: models.TaskQueued, SDUnits: &sdUnits,
				CreateTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseQueue, wantWaiter: "relay", wantReason: models.TaskAbortTimeout,
			want: now.Add(21600 * time.Second),
		},
		{
			name: "execution",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskStarted, SDUnits: &sdUnits, Timeout: 123,
				StartTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseExecution, wantWaiter: "node", wantReason: models.TaskAbortTimeout,
			want: now.Add(123 * time.Second),
		},
		{
			name: "app validation",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskScoreReady, SDUnits: &sdUnits,
				ScoreReadyTime: sql.NullTime{Time: now, Valid: true},
			},
			wantPhase: TaskTimeoutPhaseAppValidation, wantWaiter: "app", wantReason: models.TaskAbortCreatorValidationTimeout,
			want: now.Add(600 * time.Second),
		},
		{
			name: "result upload",
			task: models.InferenceTask{
				TaskType: models.TaskTypeSD, Status: models.TaskValidated, SDUnits: &sdUnits,
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

func TestUsesRelayOwnedTimeoutsRequiresCompleteWorkloadFields(t *testing.T) {
	sdUnits := uint64(512 * 512)
	inputBytes := uint64(100)
	imageCount := uint64(0)
	imagePixels := uint64(0)
	maxNewTokens := uint64(256)

	if !UsesRelayOwnedTimeouts(&models.InferenceTask{TaskType: models.TaskTypeSD, SDUnits: &sdUnits}) {
		t.Fatal("SD with SDUnits must use relay-owned timeouts")
	}
	if UsesRelayOwnedTimeouts(&models.InferenceTask{TaskType: models.TaskTypeSD}) {
		t.Fatal("SD without SDUnits must keep legacy timeouts")
	}
	if !UsesRelayOwnedTimeouts(&models.InferenceTask{
		TaskType: models.TaskTypeLLM, LLMTextInputBytes: &inputBytes, LLMImageCount: &imageCount,
		LLMImagePixels: &imagePixels, LLMMaxNewTokens: &maxNewTokens,
	}) {
		t.Fatal("LLM with all workload fields must use relay-owned timeouts")
	}
	if UsesRelayOwnedTimeouts(&models.InferenceTask{
		TaskType: models.TaskTypeLLM, LLMTextInputBytes: &inputBytes,
	}) {
		t.Fatal("LLM missing LLMMaxNewTokens must keep legacy timeouts")
	}
	if UsesRelayOwnedTimeouts(&models.InferenceTask{
		TaskType: models.TaskTypeLLM, LLMMaxNewTokens: &maxNewTokens,
	}) {
		t.Fatal("LLM missing LLMInputBytes must keep legacy timeouts")
	}
	if UsesRelayOwnedTimeouts(&models.InferenceTask{TaskType: models.TaskTypeSDFTLora, Timeout: 60}) {
		t.Fatal("SDFT must keep creator-supplied timeouts")
	}
}
