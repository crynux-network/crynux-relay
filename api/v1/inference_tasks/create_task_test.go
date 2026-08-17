package inference_tasks

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskInputTimeoutJSON(t *testing.T) {
	input := TaskInput{}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"timeout"`) {
		t.Fatalf("zero timeout must be omitted from signed input: %s", payload)
	}

	input.Timeout = 120
	payload, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"timeout":120`) {
		t.Fatalf("non-zero timeout must be included in signed input: %s", payload)
	}
}

func TestSubmitScoreExecutionDTypeJSONCompatibility(t *testing.T) {
	input := SubmitScoreInput{TaskIDCommitment: "task", Score: "0x01"}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"execution_dtype"`) {
		t.Fatalf("missing dtype must preserve old signed payload: %s", payload)
	}
	input.ExecutionDType = "bfloat16"
	payload, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"execution_dtype":"bfloat16"`) {
		t.Fatalf("reported dtype must be signed: %s", payload)
	}
}
