package models

import (
	"reflect"
	"testing"
)

func TestNormalizeModelID(t *testing.T) {
	modelID := "BaSe:Qwen/Qwen3.5-9B+FP16"
	got := NormalizeModelID(modelID)
	want := "base:qwen/qwen3.5-9b+fp16"
	if got != want {
		t.Fatalf("unexpected normalized model id, got %q, want %q", got, want)
	}
}

func TestNormalizeModelIDs(t *testing.T) {
	modelIDs := []string{
		"BaSe:Qwen/Qwen3.5-9B+FP16",
		"LoRa:Crynux-Network/MyLora+V1",
	}
	got := NormalizeModelIDs(modelIDs)
	want := []string{
		"base:qwen/qwen3.5-9b+fp16",
		"lora:crynux-network/mylora+v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized model ids, got %v, want %v", got, want)
	}
}
