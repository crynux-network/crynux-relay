package inference_tasks

import (
	"crynux_relay/models"
	"math/big"
	"path/filepath"
	"testing"
)

func TestImmutableTaskInputEqual(t *testing.T) {
	base := &models.InferenceTask{
		TaskArgs:        `{"model":"a"}`,
		Nonce:           "0x01",
		Creator:         "0xcreator",
		TaskType:        models.TaskTypeLLM,
		TaskVersion:     "1.2.3",
		MinVRAM:         16,
		RequiredGPU:     "NVIDIA RTX 4090",
		RequiredGPUVRAM: 24,
		TaskFee:         models.BigInt{Int: *big.NewInt(10)},
		TaskSize:        1,
		ModelIDs:        models.StringArray{"base:model-a"},
	}
	same := *base
	if !immutableTaskInputEqual(base, &same) {
		t.Fatal("identical immutable input must match")
	}
	conflict := same
	conflict.TaskSize = 2
	if immutableTaskInputEqual(base, &conflict) {
		t.Fatal("different immutable input must conflict")
	}
}

func TestValidatedResultPathUsesNumericEntryName(t *testing.T) {
	base := t.TempDir()
	path, err := validatedResultPath(base, 7, ".png")
	if err != nil {
		t.Fatalf("validate result path: %v", err)
	}
	if got, want := filepath.Base(path), "7.png"; got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
