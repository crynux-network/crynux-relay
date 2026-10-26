package validate

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type nodeTaskErrorGoldenInput struct {
	NodeAddress      string `json:"node_address"`
	TaskIDCommitment string `json:"task_id_commitment"`
	TaskArgs         string `json:"task_args"`
	ErrorType        string `json:"error_type"`
	Message          string `json:"message"`
	StackTrace       string `json:"stack_trace"`
	GpuCount         int    `json:"gpu_count"`
	GpuModel         string `json:"gpu_model"`
	GpuVramMb        int64  `json:"gpu_vram_mb"`
	ExecutorMode     string `json:"executor_mode"`
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

func TestValidateSignaturePreservesDecimalNumberText(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	timestamp := int64(42)
	model := json.RawMessage(
		`{"quantization_config":{"quant_kwargs":{"llm_int8_threshold":6.0,"load_in_8bit":true},"quant_method":"bitsandbytes"}}`,
	)
	input := struct {
		Address string          `json:"address"`
		Model   json.RawMessage `json:"model"`
	}{
		Address: address,
		Model:   model,
	}
	signedJSON := `{"address":"` + address + `","model":` + string(model) + `}`
	hash := crypto.Keccak256Hash(
		append([]byte(signedJSON), []byte(strconv.FormatInt(timestamp, 10))...),
	)
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("failed to sign input: %v", err)
	}

	match, recoveredAddress, err := validateSignatureAt(
		input,
		timestamp,
		hexutil.Encode(signature),
		timestamp,
	)
	if err != nil {
		t.Fatalf("failed to validate signature: %v", err)
	}
	if !match {
		t.Fatal("expected signature to validate")
	}
	if recoveredAddress != address {
		t.Fatalf("expected signer address %s, got %s", address, recoveredAddress)
	}
}
