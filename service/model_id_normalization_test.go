package service

import (
	"testing"

	"crynux_relay/models"
)

func TestMatchModelsWithNormalizedIDs(t *testing.T) {
	nodeModelIDs := models.NormalizeModelIDs([]string{
		"BaSe:Qwen/Qwen3.5-9B+FP16",
		"LoRa:Crynux-Network/MyLora+V1",
	})
	taskModelIDs := models.NormalizeModelIDs([]string{
		"base:qwen/qwen3.5-9b+fp16",
		"LORA:CRYNUX-NETWORK/MYLORA+v1",
	})

	if got := matchModels(nodeModelIDs, taskModelIDs); got != 2 {
		t.Fatalf("unexpected model match count, got %d, want %d", got, 2)
	}
	if !isSameModels(nodeModelIDs, taskModelIDs) {
		t.Fatalf("expected normalized model sets to be the same")
	}
}
