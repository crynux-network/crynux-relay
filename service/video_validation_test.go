package service

import (
	"crynux_relay/models"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"
)

func TestCheckVideoHammingDistanceTwoLevelThreshold(t *testing.T) {
	baseVisual := make([]byte, 64)
	for i := range baseVisual {
		baseVisual[i] = 0xAA
	}
	otherVisual := append([]byte{}, baseVisual...)
	// Flip 16 bits in frame 0 (d_0 = 16 < max=20), 2 bits in frames 1..7 (mean = (16 + 14)/8 = 3.75 < 10.0)
	otherVisual[0] ^= 0xFF
	otherVisual[1] ^= 0xFF
	for f := 1; f < 8; f++ {
		otherVisual[f*8] ^= 0x03
	}

	if !checkVideoHammingDistance(baseVisual, otherVisual, 81, 10.0, 20, 16) {
		t.Fatal("expected single-frame jitter of 16 bits with low mean 3.75 to match")
	}

	// Flip 20 bits in frame 0 -> hits maxThreshold=20 -> fails
	outlier := append([]byte{}, baseVisual...)
	outlier[0] ^= 0xFF
	outlier[1] ^= 0xFF
	outlier[2] ^= 0x0F
	if checkVideoHammingDistance(baseVisual, outlier, 81, 10.0, 20, 16) {
		t.Fatal("expected 20-bit single-frame outlier to fail maxThreshold=20")
	}

	// Flip 12 bits in every frame -> each < 20, but mean = 12 >= 10.0 -> fails
	highMean := append([]byte{}, baseVisual...)
	for f := 0; f < 8; f++ {
		highMean[f*8] ^= 0xFF
		highMean[f*8+1] ^= 0x0F
	}
	if checkVideoHammingDistance(baseVisual, highMean, 81, 10.0, 20, 16) {
		t.Fatal("expected mean distance 12.0 to fail meanThreshold=10.0")
	}

	// Audio block presence mismatch (64 bytes vs 72 bytes) -> fails
	withAudio1 := append(append([]byte{}, baseVisual...), 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88)
	if checkVideoHammingDistance(baseVisual, withAudio1, 81, 10.0, 20, 16) {
		t.Fatal("expected stripped audio vs audio score length mismatch to fail")
	}

	// Audio block within threshold (< 16) passes; >= 16 fails
	withAudio2 := append([]byte{}, withAudio1...)
	withAudio2[64] ^= 0x0F // 4 bits
	if !checkVideoHammingDistance(withAudio1, withAudio2, 81, 10.0, 20, 16) {
		t.Fatal("expected 4-bit audio distance to pass")
	}
	withAudioBad := append([]byte{}, withAudio1...)
	withAudioBad[64] ^= 0xFF
	withAudioBad[65] ^= 0xFF // 16 bits
	if checkVideoHammingDistance(withAudio1, withAudioBad, 81, 10.0, 20, 16) {
		t.Fatal("expected 16-bit audio distance to fail audioThreshold=16")
	}
}

func TestVideoTaskRequirementsDarwinExclusionAndPricing(t *testing.T) {
	initServiceTestConfig(t)

	videoArgs := `{
		"version": "3.8.0",
		"base_model": {"name": "Wan-AI/Wan2.2-I2V-A14B-Diffusers"},
		"dtype": "bfloat16",
		"prompt": "Rain on window",
		"image": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAQAAAACCAIAAADwyuo0AAAADklEQVR4nGNgYGBgYAAAAAUAAf7363UAAAAASUVORK5CYII=",
		"task_config": {"num_frames": 81, "steps": 4},
		"loras": [{"model": "LightX2V/Wan2.2-Lightning", "weight_file_name": "Wan2.2_High.safetensors", "weight": 100, "target": "transformer"}],
		"components": {"text_encoder": {"format": "diffusers", "repo": "Org/Custom-T5", "format_kwargs": {"subfolder": "text_encoder"}}}
	}`

	normalized, err := models.NormalizeTaskArgsModelNames(videoArgs, models.TaskTypeVideo)
	if err != nil {
		t.Fatalf("NormalizeTaskArgsModelNames error: %v", err)
	}
	modelIDs, err := models.DeriveTaskModelIDs(normalized, models.TaskTypeVideo)
	if err != nil {
		t.Fatalf("DeriveTaskModelIDs error: %v", err)
	}
	if len(modelIDs) != 3 || modelIDs[1] != "lora:lightx2v/wan2.2-lightning#Wan2.2_High.safetensors" {
		t.Fatalf("unexpected video model IDs (must preserve weight filename case): %v", modelIDs)
	}

	reqs, err := models.DeriveTaskModelRequirements(normalized, models.TaskTypeVideo)
	if err != nil {
		t.Fatalf("DeriveTaskModelRequirements error: %v", err)
	}
	if len(reqs.Entries) != 2 {
		t.Fatalf("expected 2 required model files (base_model + component), got %d", len(reqs.Entries))
	}

	task := &models.InferenceTask{
		TaskType:    models.TaskTypeVideo,
		TaskVersion: "3.8.0",
		TaskArgs:    normalized,
		MinVRAM:     24,
	}
	task.SetModelRequirements(reqs)
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("ApplyTaskPricing error: %v", err)
	}
	// Embedded PNG is 4x2 -> 4 * 2 * 81 * 4 = 2592 units
	if task.VideoUnits == nil || *task.VideoUnits != 2592 {
		t.Fatalf("expected VideoUnits=2592 from decoded base64 PNG dimensions, got %v", task.VideoUnits)
	}

	entries := []*NodeIndexEntry{
		{Address: "0xmac", Status: models.NodeStatusAvailable, GPUName: "Apple M3 Max+Darwin", GPUVram: 64, MajorVersion: 3, MinorVersion: 8, PatchVersion: 0, QOSScore: 10, HealthBase: 1.0, StakeAmount: big.NewInt(100)},
		{Address: "0xcuda", Status: models.NodeStatusAvailable, GPUName: "NVIDIA GeForce RTX 4090+Linux", GPUVram: 24, MajorVersion: 3, MinorVersion: 8, PatchVersion: 0, QOSScore: 10, HealthBase: 1.0, StakeAmount: big.NewInt(100)},
	}
	filtered := filterIndexEntriesForTask(task, entries)
	if len(filtered) != 1 || filtered[0].Address != "0xcuda" {
		t.Fatalf("expected Darwin node to be excluded for unpinned video task, got %+v", filtered)
	}

	pinnedDarwin := *task
	pinnedDarwin.RequiredGPU = "Apple M3 Max+Darwin"
	pinnedDarwin.RequiredGPUVRAM = 64
	if got := filterIndexEntriesForTask(&pinnedDarwin, entries); len(got) != 0 {
		t.Fatalf("expected Darwin node to remain excluded even when pinned on video task, got %+v", got)
	}

	// Verify compareTaskScore uses two-level video Hamming threshold
	scoreA := "0x" + hex.EncodeToString(make([]byte, 64))
	t1 := &models.InferenceTask{TaskType: models.TaskTypeVideo, Status: models.TaskScoreReady, TaskArgs: normalized, Score: scoreA}
	t2 := &models.InferenceTask{TaskType: models.TaskTypeVideo, Status: models.TaskScoreReady, TaskArgs: normalized, Score: scoreA}
	if !compareTaskScore(t1, t2, 0) {
		t.Fatal("expected identical video scores to match even when SD distance_threshold is 0")
	}

	// Non-base64 or corrupted conditioning image must fail with ErrInvalidTaskArgs
	badImageTask := &models.InferenceTask{
		TaskType: models.TaskTypeVideo,
		TaskArgs: `{"version":"3.8.0","base_model":{"name":"wan-ai/wan2.2"},"image":"https://example.com/input.png"}`,
		MinVRAM:  24,
	}
	if err := ApplyTaskPricing(badImageTask); !errors.Is(err, ErrInvalidTaskArgs) {
		t.Fatalf("expected ErrInvalidTaskArgs for non-data-URL video conditioning image, got %v", err)
	}
}
