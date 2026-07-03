package service

import (
	"crynux_relay/config"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func initTaskTraceStoreTestConfig(t *testing.T, retentionDays uint64) {
	t.Helper()
	dir := t.TempDir()
	content := "environment: test\n" +
		"blockchains: {}\n" +
		"stats:\n" +
		"  init_start_time: \"2026-01-01T00:00:00Z\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"  task_tracing_duration_days: " + strconv.FormatUint(retentionDays, 10) + "\n" +
		"qos:\n" +
		"  tracing_max_task_events: 50\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
}

func TestTaskTraceStoreDisabledDoesNotWriteRecords(t *testing.T) {
	initTaskTraceStoreTestConfig(t, 0)
	store := NewTaskTraceStore()

	store.RecordNodeSelected("0xtask", "0xnode", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	if _, ok := store.Get("0xtask", time.Now().UTC()); ok {
		t.Fatal("expected disabled tracing to skip volatile record writes")
	}
}

func TestTaskTraceStoreIndexesByTaskIDAndExpiresRecords(t *testing.T) {
	initTaskTraceStoreTestConfig(t, 1)
	store := NewTaskTraceStore()

	store.RecordValidationRequest("0xrevealed", []string{"0xtask"}, "single_task")

	record, ok := store.Get("0xtask", time.Now().UTC())
	if !ok {
		t.Fatal("expected task trace record")
	}
	if record.TaskID != "0xrevealed" {
		t.Fatalf("expected task id 0xrevealed, got %s", record.TaskID)
	}
	groupRecords := store.GetByTaskID("0xrevealed", time.Now().UTC())
	if len(groupRecords) != 1 || groupRecords[0].TaskIDCommitment != "0xtask" {
		t.Fatalf("expected one group record for 0xtask, got %#v", groupRecords)
	}

	store.CleanupExpired(time.Now().UTC().Add(48 * time.Hour))
	if _, ok := store.Get("0xtask", time.Now().UTC()); ok {
		t.Fatal("expected expired record to be removed")
	}
}
