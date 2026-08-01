package service

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"crynux_relay/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupHistoryCleanupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.InferenceTask{},
		&models.NodeTaskError{},
		&models.Event{},
		&models.PendingSlash{},
		&models.RelayAccountEvent{},
	); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	return db
}

func createTerminalTask(t *testing.T, db *gorm.DB, commitment string, status models.TaskStatus, updatedAt time.Time) *models.InferenceTask {
	t.Helper()
	task := &models.InferenceTask{
		TaskIDCommitment: commitment,
		Status:           status,
		TaskFee:          models.BigInt{Int: *big.NewInt(1)},
		Priority:         models.BigInt{Int: *big.NewInt(1)},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := db.Model(task).UpdateColumn("updated_at", updatedAt).Error; err != nil {
		t.Fatalf("failed to set updated_at: %v", err)
	}
	task.UpdatedAt = updatedAt
	return task
}

func TestCleanupTaskHistoryDeletesEligibleTaskErrorAndDirs(t *testing.T) {
	db := setupHistoryCleanupDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	cutoff := now.Add(-7 * 24 * time.Hour)

	oldCommitment := "0xold"
	createTerminalTask(t, db, oldCommitment, models.TaskEndSuccess, cutoff.Add(-time.Hour))
	createTerminalTask(t, db, "0xrecent", models.TaskEndSuccess, cutoff.Add(time.Hour))
	createTerminalTask(t, db, "0xrunning", models.TaskStarted, cutoff.Add(-time.Hour))

	if err := db.Create(&models.NodeTaskError{
		NodeAddress:      "0xnode",
		TaskIDCommitment: oldCommitment,
		TaskArgs:         "{}",
		ErrorType:        "runtime",
		Message:          "boom",
		StackTrace:       "stack",
		CapturedAt:       now.Unix(),
	}).Error; err != nil {
		t.Fatalf("failed to create node task error: %v", err)
	}
	if err := db.Create(&models.Event{
		Type:             "TaskEndSuccess",
		NodeAddress:      "0xnode",
		TaskIDCommitment: oldCommitment,
		Args:             "{}",
	}).Error; err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	inferenceDir := t.TempDir()
	slashedDir := t.TempDir()
	oldInferencePath := filepath.Join(inferenceDir, oldCommitment)
	oldSlashedPath := filepath.Join(slashedDir, oldCommitment)
	if err := os.MkdirAll(filepath.Join(oldInferencePath, "results"), 0o755); err != nil {
		t.Fatalf("failed to create inference dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(oldSlashedPath, "results"), 0o755); err != nil {
		t.Fatalf("failed to create slashed dir: %v", err)
	}

	if err := CleanupTaskHistory(ctx, db, cutoff, inferenceDir, slashedDir, 100); err != nil {
		t.Fatalf("CleanupTaskHistory failed: %v", err)
	}

	var oldCount int64
	if err := db.Unscoped().Model(&models.InferenceTask{}).Where("task_id_commitment = ?", oldCommitment).Count(&oldCount).Error; err != nil {
		t.Fatalf("failed to count old task: %v", err)
	}
	if oldCount != 0 {
		t.Fatalf("expected old terminal task deleted, got count %d", oldCount)
	}

	var retained int64
	if err := db.Model(&models.InferenceTask{}).Count(&retained).Error; err != nil {
		t.Fatalf("failed to count retained tasks: %v", err)
	}
	if retained != 2 {
		t.Fatalf("expected 2 retained tasks, got %d", retained)
	}

	var errorCount int64
	if err := db.Model(&models.NodeTaskError{}).Where("task_id_commitment = ?", oldCommitment).Count(&errorCount).Error; err != nil {
		t.Fatalf("failed to count node task errors: %v", err)
	}
	if errorCount != 0 {
		t.Fatalf("expected node task error deleted, got count %d", errorCount)
	}

	var eventCount int64
	if err := db.Model(&models.Event{}).Where("task_id_commitment = ?", oldCommitment).Count(&eventCount).Error; err != nil {
		t.Fatalf("failed to count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected task cleanup to leave events untouched, got count %d", eventCount)
	}

	if _, err := os.Stat(oldInferencePath); !os.IsNotExist(err) {
		t.Fatalf("expected inference artifact dir removed, stat err=%v", err)
	}
	if _, err := os.Stat(oldSlashedPath); !os.IsNotExist(err) {
		t.Fatalf("expected slashed artifact dir removed, stat err=%v", err)
	}
}

func TestCleanupTaskHistorySkipsPendingSlashAndPendingLedger(t *testing.T) {
	db := setupHistoryCleanupDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	cutoff := now.Add(-7 * 24 * time.Hour)
	oldUpdatedAt := cutoff.Add(-time.Hour)

	slashCommitment := "0xslash"
	ledgerCommitment := "0xledger"
	createTerminalTask(t, db, slashCommitment, models.TaskEndAborted, oldUpdatedAt)
	createTerminalTask(t, db, ledgerCommitment, models.TaskEndSuccess, oldUpdatedAt)

	if err := db.Create(&models.PendingSlash{
		Status:           models.PendingSlashStatusPending,
		NodeAddress:      "0xnode",
		Network:          "base",
		TaskIDCommitment: slashCommitment,
		EvidenceJSON:     "{}",
	}).Error; err != nil {
		t.Fatalf("failed to create pending slash: %v", err)
	}

	if err := db.Create(&models.RelayAccountEvent{
		CreatedAt: now,
		Address:   "0xcreator",
		Amount:    models.BigInt{Int: *big.NewInt(1)},
		Status:    models.RelayAccountEventStatusPending,
		Reason:    fmt.Sprintf("%d-%s", models.RelayAccountEventTypeTaskPayment, ledgerCommitment),
		Type:      models.RelayAccountEventTypeTaskPayment,
		MAC:       "mac",
	}).Error; err != nil {
		t.Fatalf("failed to create pending ledger event: %v", err)
	}

	if err := CleanupTaskHistory(ctx, db, cutoff, t.TempDir(), t.TempDir(), 100); err != nil {
		t.Fatalf("CleanupTaskHistory failed: %v", err)
	}

	var count int64
	if err := db.Model(&models.InferenceTask{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count tasks: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected blocked tasks retained, got count %d", count)
	}
}

func TestCleanupEventHistoryDeletesOldEventsOnly(t *testing.T) {
	db := setupHistoryCleanupDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	cutoff := now.Add(-7 * 24 * time.Hour)

	oldJoin := &models.Event{Type: "NodeJoin", NodeAddress: "0xnode", TaskIDCommitment: "", Args: "{}"}
	oldTask := &models.Event{Type: "TaskStarted", NodeAddress: "0xnode", TaskIDCommitment: "0xtask", Args: "{}"}
	recent := &models.Event{Type: "NodeJoin", NodeAddress: "0xnode2", TaskIDCommitment: "", Args: "{}"}
	for _, event := range []*models.Event{oldJoin, oldTask, recent} {
		if err := db.Create(event).Error; err != nil {
			t.Fatalf("failed to create event: %v", err)
		}
	}
	if err := db.Model(&models.Event{}).Where("id IN ?", []uint{oldJoin.ID, oldTask.ID}).
		UpdateColumn("created_at", cutoff.Add(-time.Hour)).Error; err != nil {
		t.Fatalf("failed to age old events: %v", err)
	}
	if err := db.Model(&models.Event{}).Where("id = ?", recent.ID).
		UpdateColumn("created_at", cutoff.Add(time.Hour)).Error; err != nil {
		t.Fatalf("failed to set recent event created_at: %v", err)
	}

	task := createTerminalTask(t, db, "0xtask", models.TaskEndSuccess, cutoff.Add(-time.Hour))

	if err := CleanupEventHistory(ctx, db, cutoff, 100); err != nil {
		t.Fatalf("CleanupEventHistory failed: %v", err)
	}

	var eventCount int64
	if err := db.Unscoped().Model(&models.Event{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("failed to count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected 1 retained event, got %d", eventCount)
	}
	var retained models.Event
	if err := db.First(&retained).Error; err != nil {
		t.Fatalf("failed to load retained event: %v", err)
	}
	if retained.ID != recent.ID {
		t.Fatalf("expected recent event retained, got id %d", retained.ID)
	}

	var taskCount int64
	if err := db.Model(&models.InferenceTask{}).Where("id = ?", task.ID).Count(&taskCount).Error; err != nil {
		t.Fatalf("failed to count task: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("expected event cleanup to leave inference_tasks untouched")
	}
}
