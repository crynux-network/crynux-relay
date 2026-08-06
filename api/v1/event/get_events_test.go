package event

import (
	"crynux_relay/models"
	"encoding/json"
	"testing"
)

func taskEndAbortedEventRow(t *testing.T, reason models.TaskAbortReason) *models.Event {
	t.Helper()
	payload := &models.TaskEndAbortedEvent{
		TaskIDCommitment: "0x01",
		AbortIssuer:      "0x02",
		LastStatus:       models.TaskScoreReady,
		AbortReason:      reason,
	}
	row, err := payload.ToEvent()
	if err != nil {
		t.Fatalf("build TaskEndAborted event: %v", err)
	}
	return row
}

func TestCompatibleEventArgsRewritesLaterAbortReasons(t *testing.T) {
	for _, reason := range []models.TaskAbortReason{
		models.TaskAbortGroupTimeout,
		models.TaskAbortErrorReported,
		models.TaskAbortCreatorCancelled,
		models.TaskAbortCreatorValidationTimeout,
		models.TaskAbortResultUploadTimeout,
		models.TaskAbortNodeSlashed,
	} {
		row := taskEndAbortedEventRow(t, reason)
		var args models.TaskEndAbortedEvent
		if err := json.Unmarshal([]byte(compatibleEventArgs(row)), &args); err != nil {
			t.Fatalf("parse rewritten args for reason %d: %v", reason, err)
		}
		if args.AbortReason != models.TaskAbortTimeout {
			t.Fatalf("expected reason %d to be delivered as TaskAbortTimeout, got %d", reason, args.AbortReason)
		}
		if args.TaskIDCommitment != "0x01" || args.AbortIssuer != "0x02" || args.LastStatus != models.TaskScoreReady {
			t.Fatalf("rewritten args changed unrelated fields: %+v", args)
		}
	}
}

func TestCompatibleEventArgsKeepsNodeParsableAbortReasons(t *testing.T) {
	for _, reason := range []models.TaskAbortReason{
		models.TaskAbortReasonNone,
		models.TaskAbortTimeout,
		models.TaskAbortModelDownloadFailed,
		models.TaskAbortIncorrectResult,
		models.TaskAbortTaskFeeTooLow,
	} {
		row := taskEndAbortedEventRow(t, reason)
		if got := compatibleEventArgs(row); got != row.Args {
			t.Fatalf("expected args for reason %d to stay unchanged, got %s", reason, got)
		}
	}
}

func TestCompatibleEventArgsKeepsOtherEventTypes(t *testing.T) {
	payload := &models.TaskStartedEvent{
		TaskIDCommitment: "0x01",
		SelectedNode:     "0x02",
	}
	row, err := payload.ToEvent()
	if err != nil {
		t.Fatalf("build TaskStarted event: %v", err)
	}
	if got := compatibleEventArgs(row); got != row.Args {
		t.Fatalf("expected non-TaskEndAborted args to stay unchanged, got %s", got)
	}
}

func TestCompatibleEventArgsKeepsUnparsableArgs(t *testing.T) {
	row := &models.Event{Type: "TaskEndAborted", Args: "not json"}
	if got := compatibleEventArgs(row); got != row.Args {
		t.Fatalf("expected unparsable args to stay unchanged, got %s", got)
	}
}
