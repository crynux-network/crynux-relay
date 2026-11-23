package admin

import (
	"crynux_relay/models"
	"database/sql"
	"testing"
	"time"
)

func TestTaskTraceCreatedIncludesVideoUnits(t *testing.T) {
	videoUnits := uint64(832 * 480 * 81 * 30)
	task := &models.InferenceTask{
		TaskIDCommitment: "0xvideotrace",
		TaskType:         models.TaskTypeVideo,
		TaskVersion:      "3.8.0",
		Status:           models.TaskQueued,
		ModelName:        "wan-ai/wan2.2-t2v-a14b-diffusers",
		RequestedDType:   "bfloat16",
		MinVRAM:          24,
		TaskSize:         1,
		VideoUnits:       &videoUnits,
		CreateTime:       sql.NullTime{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
	}

	builder := newTaskTraceBuilder(task, nil, nil, nil, true, false)
	builder.addTaskCreated()
	if len(builder.trace) != 1 {
		t.Fatalf("expected 1 trace step, got %d", len(builder.trace))
	}
	got, ok := builder.trace[0].Details["video_units"].(uint64)
	if !ok || got != videoUnits {
		t.Fatalf("expected video_units=%d in task_created details, got %#v", videoUnits, builder.trace[0].Details["video_units"])
	}
	if builder.trace[0].Details["timeout_ownership"] != "relay" {
		t.Fatalf("expected relay timeout_ownership for video task, got %#v", builder.trace[0].Details["timeout_ownership"])
	}
}
