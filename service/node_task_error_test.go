package service

import (
	"context"
	"crynux_relay/models"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newNodeTaskErrorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeTaskError{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestCreateNodeTaskErrorIsIdempotent(t *testing.T) {
	db := newNodeTaskErrorTestDB(t)
	record := &models.NodeTaskError{
		NodeAddress:      "0x1111111111111111111111111111111111111111",
		TaskIDCommitment: "task-1",
		TaskArgs:         `{"prompt":"first"}`,
		ErrorType:        "TaskExecutionError",
		Message:          "execution failed",
		StackTrace:       "Traceback: original",
		CapturedAt:       1_721_234_567,
	}

	created, err := CreateNodeTaskError(context.Background(), db, record)
	if err != nil || !created {
		t.Fatalf("expected first insert to create a record, created=%v err=%v", created, err)
	}
	duplicate := *record
	duplicate.ID = 0
	duplicate.Message = "retry payload"
	created, err = CreateNodeTaskError(context.Background(), db, &duplicate)
	if err != nil || created {
		t.Fatalf("expected duplicate insert to succeed without creation, created=%v err=%v", created, err)
	}

	var records []models.NodeTaskError
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("failed to query records: %v", err)
	}
	if len(records) != 1 || records[0].Message != "execution failed" {
		t.Fatalf("expected original record to remain unchanged, got %+v", records)
	}
}

func TestListNodeTaskErrorsUsesExactFiltersAndReversePagination(t *testing.T) {
	db := newNodeTaskErrorTestDB(t)
	baseTime := time.Date(2026, 7, 24, 1, 0, 0, 0, time.UTC)
	records := []models.NodeTaskError{
		{
			CreatedAt:        baseTime,
			NodeAddress:      "0x1111111111111111111111111111111111111111",
			TaskIDCommitment: "task-1",
		},
		{
			CreatedAt:        baseTime.Add(time.Minute),
			NodeAddress:      "0x1111111111111111111111111111111111111111",
			TaskIDCommitment: "task-2",
		},
		{
			CreatedAt:        baseTime.Add(2 * time.Minute),
			NodeAddress:      "0x2222222222222222222222222222222222222222",
			TaskIDCommitment: "task-3",
		},
	}
	for i := range records {
		records[i].TaskArgs = fmt.Sprintf(`{"index":%d}`, i)
		records[i].ErrorType = "TaskExecutionError"
		records[i].Message = "failed"
		records[i].StackTrace = "trace"
		records[i].CapturedAt = int64(i + 1)
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create record: %v", err)
		}
	}

	page, total, err := ListNodeTaskErrors(context.Background(), db, NodeTaskErrorFilter{}, 1, 2)
	if err != nil {
		t.Fatalf("failed to list records: %v", err)
	}
	if total != 3 || len(page) != 2 || page[0].TaskIDCommitment != "task-3" || page[1].TaskIDCommitment != "task-2" {
		t.Fatalf("unexpected reverse page: total=%d records=%+v", total, page)
	}

	filtered, total, err := ListNodeTaskErrors(context.Background(), db, NodeTaskErrorFilter{
		NodeAddress:      records[0].NodeAddress,
		TaskIDCommitment: "task-2",
	}, 1, 10)
	if err != nil {
		t.Fatalf("failed to filter records: %v", err)
	}
	if total != 1 || len(filtered) != 1 || filtered[0].TaskIDCommitment != "task-2" {
		t.Fatalf("unexpected combined filter result: total=%d records=%+v", total, filtered)
	}

	partial, total, err := ListNodeTaskErrors(context.Background(), db, NodeTaskErrorFilter{
		TaskIDCommitment: "task",
	}, 1, 10)
	if err != nil {
		t.Fatalf("failed to apply exact filter: %v", err)
	}
	if total != 0 || len(partial) != 0 {
		t.Fatalf("expected partial value not to match, total=%d records=%+v", total, partial)
	}
}
