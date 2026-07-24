package admin

import (
	"context"
	"crynux_relay/models"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAdminNodeTaskErrorTestDB(t *testing.T) *gorm.DB {
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

func TestListNodeTaskErrorsReturnsFilteredReversePageWithCompleteContent(t *testing.T) {
	db := newAdminNodeTaskErrorTestDB(t)
	nodeAddress := "0x1111111111111111111111111111111111111111"
	createdAt := time.Date(2026, 7, 24, 2, 0, 0, 0, time.UTC)
	for i := 1; i <= 3; i++ {
		record := models.NodeTaskError{
			CreatedAt:        createdAt,
			NodeAddress:      nodeAddress,
			TaskIDCommitment: fmt.Sprintf("task-%d", i),
			TaskArgs:         fmt.Sprintf(`{"prompt":"task %d"}`, i),
			ErrorType:        "TaskExecutionError",
			Message:          fmt.Sprintf("failure %d", i),
			StackTrace:       fmt.Sprintf("complete trace %d", i),
			CapturedAt:       int64(i),
		}
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("failed to create record: %v", err)
		}
	}

	result, err := listNodeTaskErrors(context.Background(), db, &ListNodeTaskErrorsInput{
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("failed to list node task errors: %v", err)
	}
	if result.Data.Total != 3 || result.Data.Page != 1 || result.Data.PageSize != 2 || len(result.Data.Items) != 2 {
		t.Fatalf("unexpected pagination data: %+v", result.Data)
	}
	if result.Data.Items[0].TaskIDCommitment != "task-3" || result.Data.Items[1].TaskIDCommitment != "task-2" {
		t.Fatalf("expected id-desc tie break, got %+v", result.Data.Items)
	}
	if result.Data.Items[0].TaskArgs != `{"prompt":"task 3"}` || result.Data.Items[0].StackTrace != "complete trace 3" {
		t.Fatalf("expected complete task args and stack trace, got %+v", result.Data.Items[0])
	}

	combined, err := listNodeTaskErrors(context.Background(), db, &ListNodeTaskErrorsInput{
		NodeAddress:      nodeAddress,
		TaskIDCommitment: "task-1",
		Page:             1,
		PageSize:         200,
	})
	if err != nil {
		t.Fatalf("failed to apply combined filters: %v", err)
	}
	if combined.Data.Total != 1 || len(combined.Data.Items) != 1 || combined.Data.PageSize != maxNodeTaskErrorPageSize {
		t.Fatalf("unexpected combined filter result: %+v", combined.Data)
	}
}

func TestClampNodeTaskErrorPaginationUsesDefaultsAndMaximum(t *testing.T) {
	page, pageSize := clampNodeTaskErrorPagination(0, 0)
	if page != 1 || pageSize != defaultNodeTaskErrorPageSize {
		t.Fatalf("unexpected defaults page=%d pageSize=%d", page, pageSize)
	}
	_, pageSize = clampNodeTaskErrorPagination(1, maxNodeTaskErrorPageSize+1)
	if pageSize != maxNodeTaskErrorPageSize {
		t.Fatalf("expected max page size %d, got %d", maxNodeTaskErrorPageSize, pageSize)
	}
}
