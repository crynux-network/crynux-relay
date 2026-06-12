package service

import (
	"context"
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
			TaskIDCommitment: "expired-queued",
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-5 * time.Minute), Valid: true},
			Timeout:          60,
		},
		{
			TaskIDCommitment: "active-queued",
			Status:           models.TaskQueued,
			CreateTime:       sql.NullTime{Time: now.Add(-30 * time.Second), Valid: true},
			Timeout:          60,
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

	if len(timedOutTasks) != 1 {
		t.Fatalf("expected one timed out queued task, got %d", len(timedOutTasks))
	}
	if timedOutTasks[0].TaskIDCommitment != "expired-queued" {
		t.Fatalf("unexpected timed out queued task: %s", timedOutTasks[0].TaskIDCommitment)
	}
}
