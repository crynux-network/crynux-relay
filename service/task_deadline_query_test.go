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

func TestGetDueTaskDeadlinesIsBoundedAndOrdered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("migrate inference tasks: %v", err)
	}
	now := time.Now()
	tasks := []models.InferenceTask{
		{TaskIDCommitment: "later", Status: models.TaskStarted, DeadlineAt: sql.NullTime{Time: now.Add(-time.Second), Valid: true}},
		{TaskIDCommitment: "first", Status: models.TaskStarted, DeadlineAt: sql.NullTime{Time: now.Add(-2 * time.Second), Valid: true}},
		{TaskIDCommitment: "future", Status: models.TaskStarted, DeadlineAt: sql.NullTime{Time: now.Add(time.Hour), Valid: true}},
		{TaskIDCommitment: "terminal", Status: models.TaskEndSuccess, DeadlineAt: sql.NullTime{Time: now.Add(-time.Hour), Valid: true}},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	due, err := getDueTaskDeadlines(context.Background(), db, now, 1, nil)
	if err != nil {
		t.Fatalf("query due tasks: %v", err)
	}
	if len(due) != 1 || due[0].TaskIDCommitment != "first" {
		t.Fatalf("expected only first due task, got %#v", due)
	}
	cursor := &taskDeadlineCursor{Status: due[0].Status, DeadlineAt: due[0].DeadlineAt.Time, ID: due[0].ID}
	next, err := getDueTaskDeadlines(context.Background(), db, now, 1, cursor)
	if err != nil {
		t.Fatalf("query next due task: %v", err)
	}
	if len(next) != 1 || next[0].TaskIDCommitment != "later" {
		t.Fatalf("expected next keyset task, got %#v", next)
	}
}
