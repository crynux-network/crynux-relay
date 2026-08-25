package models

import (
	"context"
	"math/big"
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

func mustBigInt(t *testing.T, value string) BigInt {
	t.Helper()
	z := new(big.Int)
	if _, ok := z.SetString(value, 10); !ok {
		t.Fatalf("invalid big int %q", value)
	}
	return BigInt{Int: *z}
}

func createQueuedTask(t *testing.T, db *gorm.DB, taskType TaskType, priority string, commitment string) {
	t.Helper()
	task := InferenceTask{
		TaskIDCommitment: commitment,
		Status:           TaskQueued,
		TaskType:         taskType,
		Priority:         mustBigInt(t, priority),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
}

func TestGetQueuedTaskPriorityRangeEmpty(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	result, err := GetQueuedTaskPriorityRange(context.Background(), db)
	if err != nil {
		t.Fatalf("GetQueuedTaskPriorityRange: %v", err)
	}
	if result.Count != 0 || result.Highest != nil || result.Median != nil || result.Lowest != nil {
		t.Fatalf("expected empty range, got %+v", result)
	}
}

func TestGetQueuedTaskPriorityRangeIncludesSDAndLLMExcludesNonQueued(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	createQueuedTask(t, db, TaskTypeLLM, "100000000000", "llm-high")
	createQueuedTask(t, db, TaskTypeSD, "80000000000", "sd-mid")
	createQueuedTask(t, db, TaskTypeLLM, "40000000000", "llm-low")
	started := InferenceTask{
		TaskIDCommitment: "started",
		Status:           TaskStarted,
		TaskType:         TaskTypeLLM,
		Priority:         mustBigInt(t, "999999999999"),
	}
	if err := db.Create(&started).Error; err != nil {
		t.Fatalf("failed to create started task: %v", err)
	}

	result, err := GetQueuedTaskPriorityRange(context.Background(), db)
	if err != nil {
		t.Fatalf("GetQueuedTaskPriorityRange: %v", err)
	}
	if result.Count != 3 {
		t.Fatalf("expected count 3, got %d", result.Count)
	}
	if result.Highest.String() != "100000000000" {
		t.Fatalf("expected highest 100000000000, got %s", result.Highest.String())
	}
	if result.Median.String() != "80000000000" {
		t.Fatalf("expected median 80000000000, got %s", result.Median.String())
	}
	if result.Lowest.String() != "40000000000" {
		t.Fatalf("expected lowest 40000000000, got %s", result.Lowest.String())
	}
}

func TestGetQueuedTaskPriorityRangeOddAndEvenMedian(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	createQueuedTask(t, db, TaskTypeLLM, "100", "a")
	createQueuedTask(t, db, TaskTypeLLM, "80", "b")
	createQueuedTask(t, db, TaskTypeLLM, "60", "c")
	createQueuedTask(t, db, TaskTypeLLM, "40", "d")

	evenResult, err := GetQueuedTaskPriorityRange(context.Background(), db)
	if err != nil {
		t.Fatalf("even GetQueuedTaskPriorityRange: %v", err)
	}
	if evenResult.Median.String() != "60" {
		t.Fatalf("expected even median 60, got %s", evenResult.Median.String())
	}

	createQueuedTask(t, db, TaskTypeSD, "20", "e")
	oddResult, err := GetQueuedTaskPriorityRange(context.Background(), db)
	if err != nil {
		t.Fatalf("odd GetQueuedTaskPriorityRange: %v", err)
	}
	if oddResult.Count != 5 {
		t.Fatalf("expected count 5, got %d", oddResult.Count)
	}
	if oddResult.Median.String() != "60" {
		t.Fatalf("expected odd median 60, got %s", oddResult.Median.String())
	}
}

func TestGetQueuedTaskPriorityRangeSingleTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	createQueuedTask(t, db, TaskTypeLLM, "42", "only")
	result, err := GetQueuedTaskPriorityRange(context.Background(), db)
	if err != nil {
		t.Fatalf("GetQueuedTaskPriorityRange: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("expected count 1, got %d", result.Count)
	}
	if result.Highest.String() != "42" || result.Median.String() != "42" || result.Lowest.String() != "42" {
		t.Fatalf("expected all priorities 42, got highest=%s median=%s lowest=%s",
			result.Highest.String(), result.Median.String(), result.Lowest.String())
	}
}
