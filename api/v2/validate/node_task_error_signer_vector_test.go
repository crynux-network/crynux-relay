package validate

import (
	"encoding/json"
	"os"
	"testing"
)

type nodeTaskErrorGoldenInput struct {
	NodeAddress      string `json:"node_address"`
	TaskIDCommitment string `json:"task_id_commitment"`
	TaskArgs         string `json:"task_args"`
	ErrorType        string `json:"error_type"`
	Message          string `json:"message"`
	StackTrace       string `json:"stack_trace"`
}

func TestPythonNodeTaskErrorSignerGoldenVector(t *testing.T) {
	data, err := os.ReadFile("testdata/node_task_error_signer_vector.json")
	if err != nil {
		t.Fatalf("failed to read signer vector: %v", err)
	}
	var vector struct {
		Input      nodeTaskErrorGoldenInput `json:"input"`
		CapturedAt int64                    `json:"captured_at"`
		Timestamp  int64                    `json:"timestamp"`
		Signature  string                   `json:"signature"`
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatalf("failed to decode signer vector: %v", err)
	}

	match, address, err := validateSignatureAt(vector.Input, vector.Timestamp, vector.Signature, vector.Timestamp)
	if err != nil {
		t.Fatalf("failed to validate Python signer vector: %v", err)
	}
	if !match {
		t.Fatal("expected Python signer vector to validate")
	}
	if address != vector.Input.NodeAddress {
		t.Fatalf("expected signer address %s, got %s", vector.Input.NodeAddress, address)
	}
	if vector.CapturedAt == 0 {
		t.Fatal("expected unsigned captured_at in the request envelope vector")
	}
}

func TestValidateSignatureRejectsMalformedSignature(t *testing.T) {
	match, _, err := validateSignatureAt(struct{}{}, 1, "bad", 1)
	if err == nil || match {
		t.Fatalf("expected malformed signature to fail safely, match=%v err=%v", match, err)
	}
}
