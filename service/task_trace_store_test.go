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
		"http:\n" +
		"  max_body_bytes: 33554432\n" +
		"stats:\n" +
		"  init_start_time: \"2026-01-01T00:00:00Z\"\n" +
		"network_flops:\n" +
		"  gpu_flops_file: \"config/gpu_flops.json\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"  task_tracing_duration_days: " + strconv.FormatUint(retentionDays, 10) + "\n" +
		taskPricingMatchingTestConfigYAML +
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

	store.RecordNodeSelected(
		"0xtask",
		"0xnode",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		[]TaskTraceNodeSelectionCandidate{{Address: "0xnode", CardName: "RTX 4090", StakingScore: 0.5, QOSScore: 0.8, ProbWeight: 0.31}},
		1,
		false,
	)

	if _, ok := store.Get("0xtask", time.Now().UTC()); ok {
		t.Fatal("expected disabled tracing to skip volatile record writes")
	}
}

func TestTaskTraceStoreRecordsNodeSelectionCandidatePool(t *testing.T) {
	initTaskTraceStoreTestConfig(t, 1)
	store := NewTaskTraceStore()
	candidatePool := []TaskTraceNodeSelectionCandidate{
		{Address: "0xnode1", CardName: "RTX 4090", StakingScore: 0.5, QOSScore: 0.8, ProbWeight: 0.31},
		{Address: "0xnode2", CardName: "A100", StakingScore: 1, QOSScore: 0.9, ProbWeight: 0.47},
	}

	store.RecordNodeSelected("0xtask", "0xnode2", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), candidatePool, len(candidatePool), false)

	record, ok := store.Get("0xtask", time.Now().UTC())
	if !ok {
		t.Fatal("expected task trace record")
	}
	if record.SelectedNode != "0xnode2" {
		t.Fatalf("expected selected node 0xnode2, got %s", record.SelectedNode)
	}
	if record.NodeSelectionCandidatePoolTotalCount != 2 {
		t.Fatalf("expected total count 2, got %d", record.NodeSelectionCandidatePoolTotalCount)
	}
	if record.NodeSelectionCandidatePoolTruncated {
		t.Fatal("expected candidate pool not to be truncated")
	}
	if len(record.NodeSelectionCandidatePool) != 2 {
		t.Fatalf("expected two candidate pool records, got %d", len(record.NodeSelectionCandidatePool))
	}
	if record.NodeSelectionCandidatePool[1].ProbWeight != 0.47 {
		t.Fatalf("expected final prob weight 0.47, got %f", record.NodeSelectionCandidatePool[1].ProbWeight)
	}

	candidatePool[0].Address = "0xmutated"
	record.NodeSelectionCandidatePool[1].Address = "0xmutated"

	recordAgain, ok := store.Get("0xtask", time.Now().UTC())
	if !ok {
		t.Fatal("expected task trace record")
	}
	if recordAgain.NodeSelectionCandidatePool[0].Address != "0xnode1" || recordAgain.NodeSelectionCandidatePool[1].Address != "0xnode2" {
		t.Fatalf("expected candidate pool to be cloned, got %#v", recordAgain.NodeSelectionCandidatePool)
	}
}

func TestTaskTraceStoreCapsNodeSelectionCandidatePool(t *testing.T) {
	initTaskTraceStoreTestConfig(t, 1)
	store := NewTaskTraceStore()
	candidatePool := make([]TaskTraceNodeSelectionCandidate, 0, taskTraceCandidatePoolLimit+1)
	for i := 0; i < taskTraceCandidatePoolLimit+1; i++ {
		candidatePool = append(candidatePool, TaskTraceNodeSelectionCandidate{Address: "0xnode" + strconv.Itoa(i)})
	}

	store.RecordNodeSelected("0xtask", "0xnode0", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), candidatePool, len(candidatePool), false)

	record, ok := store.Get("0xtask", time.Now().UTC())
	if !ok {
		t.Fatal("expected task trace record")
	}
	if len(record.NodeSelectionCandidatePool) != taskTraceCandidatePoolLimit {
		t.Fatalf("expected capped candidate pool size %d, got %d", taskTraceCandidatePoolLimit, len(record.NodeSelectionCandidatePool))
	}
	if record.NodeSelectionCandidatePoolTotalCount != taskTraceCandidatePoolLimit+1 {
		t.Fatalf("expected total count %d, got %d", taskTraceCandidatePoolLimit+1, record.NodeSelectionCandidatePoolTotalCount)
	}
	if !record.NodeSelectionCandidatePoolTruncated {
		t.Fatal("expected candidate pool to be marked truncated")
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
