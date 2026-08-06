package models

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInferenceTaskSyncStatusRefreshesAbortReason(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	storedTask := InferenceTask{
		TaskIDCommitment: "commitment",
		Status:           TaskEndAborted,
		AbortReason:      TaskAbortCreatorValidationTimeout,
	}
	if err := db.Create(&storedTask).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	staleTask := InferenceTask{
		Model:       gorm.Model{ID: storedTask.ID},
		Status:      TaskScoreReady,
		AbortReason: TaskAbortReasonNone,
	}
	if err := staleTask.SyncStatus(context.Background(), db); err != nil {
		t.Fatalf("failed to sync task status: %v", err)
	}
	if staleTask.Status != TaskEndAborted {
		t.Fatalf("expected aborted status, got %v", staleTask.Status)
	}
	if staleTask.AbortReason != TaskAbortCreatorValidationTimeout {
		t.Fatalf("expected creator validation timeout reason, got %v", staleTask.AbortReason)
	}
}

func TestTaskAbortReasonValuesAreAppended(t *testing.T) {
	if TaskAbortResultUploadTimeout != 9 {
		t.Fatalf("existing abort reason value changed: got %d", TaskAbortResultUploadTimeout)
	}
	if TaskAbortNodeSlashed != 10 {
		t.Fatalf("node-slashed abort reason was not appended: got %d", TaskAbortNodeSlashed)
	}
}
