package service

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func initQosTracingTestConfig(t *testing.T, maxEvents uint64) {
	t.Helper()
	dir := t.TempDir()
	content := "environment: test\n" +
		"blockchains: {}\n" +
		"http:\n" +
		"  jwt:\n" +
		"    expires_in: 3600\n" +
		"stats:\n" +
		"  init_start_time: \"2026-01-01T00:00:00Z\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"qos:\n" +
		"  tracing_max_task_events: " + uint64ToString(maxEvents) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
}

func resetQosTraceStoreForTest() {
	qosTraceStore.mu.Lock()
	defer qosTraceStore.mu.Unlock()
	qosTraceStore.events = make(map[string][]NodeQosTraceEvent)
}

func TestRecordNodeQosTraceRetainsNewestEventsPerNode(t *testing.T) {
	initQosTracingTestConfig(t, 2)
	resetQosTraceStoreForTest()

	for i := 0; i < 3; i++ {
		RecordNodeQosTrace(NodeQosTraceInput{
			NodeAddress:      "0xnode",
			TaskIDCommitment: "0xtask" + uint64ToString(uint64(i)),
			EventType:        QosTraceEventTaskTimeoutPenalty,
			Before:           NodeQosTraceValues{QosLong: 0.5, QosShort: 1.0, Qos: 0.5},
			After:            NodeQosTraceValues{QosLong: 0.5, QosShort: 0.9 - float64(i)*0.1, Qos: 0.45},
		})
	}

	events := ListNodeQosTraceEvents("0xnode")
	if len(events) != 2 {
		t.Fatalf("expected 2 retained events, got %d", len(events))
	}
	if events[0].TaskIDCommitment != "0xtask1" || events[1].TaskIDCommitment != "0xtask2" {
		t.Fatalf("unexpected retained events: %+v", events)
	}
}

func TestRecordNodeQosTraceSkipsUnchangedValues(t *testing.T) {
	initQosTracingTestConfig(t, 2)
	resetQosTraceStoreForTest()

	values := NodeQosTraceValues{QosLong: 0.5, QosShort: 1.0, Qos: 0.5}
	RecordNodeQosTrace(NodeQosTraceInput{
		NodeAddress: "0xnode",
		EventType:   QosTraceEventTaskResultUploadSuccessBoost,
		Before:      values,
		After:       values,
	})

	if events := ListNodeQosTraceEvents("0xnode"); len(events) != 0 {
		t.Fatalf("expected unchanged trace to be skipped, got %+v", events)
	}
}

func TestBuildValidationGroupQosTraceMetadata(t *testing.T) {
	tests := []struct {
		score     int64
		eventType string
		rank      *uint64
	}{
		{score: 10, eventType: QosTraceEventValidationGroupRank1, rank: uint64Ptr(1)},
		{score: 5, eventType: QosTraceEventValidationGroupRank2, rank: uint64Ptr(2)},
		{score: 2, eventType: QosTraceEventValidationGroupRank3, rank: uint64Ptr(3)},
		{score: 0, eventType: QosTraceEventValidationGroupAborted, rank: nil},
	}

	for _, tt := range tests {
		task := &models.InferenceTask{QOSScore: sql.NullInt64{Int64: tt.score, Valid: true}}
		eventType, rank := BuildValidationGroupQosTraceMetadata(task)
		if eventType != tt.eventType {
			t.Fatalf("score %d expected event %s, got %s", tt.score, tt.eventType, eventType)
		}
		if tt.rank == nil {
			if rank != nil {
				t.Fatalf("score %d expected no rank, got %d", tt.score, *rank)
			}
			continue
		}
		if rank == nil || *rank != *tt.rank {
			t.Fatalf("score %d expected rank %d, got %v", tt.score, *tt.rank, rank)
		}
	}
}

func TestCaptureNodeQosTraceValuesUsesEffectiveHealth(t *testing.T) {
	initQosTracingTestConfig(t, 2)
	node := &models.Node{
		QOSScore:        5,
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true},
	}

	values := CaptureNodeQosTraceValues(node)
	if values.QosLong != 0.5 {
		t.Fatalf("expected long-term qos 0.5, got %.4f", values.QosLong)
	}
	if values.QosShort <= 0.5 {
		t.Fatalf("expected passive recovery to affect captured short-term qos, got %.4f", values.QosShort)
	}
	if events := ListNodeQosTraceEvents(node.Address); len(events) != 0 {
		t.Fatalf("passive capture should not create trace events, got %+v", events)
	}
}

func uint64ToString(v uint64) string {
	return strconv.FormatUint(v, 10)
}
