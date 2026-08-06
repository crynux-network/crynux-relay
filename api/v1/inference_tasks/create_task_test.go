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
