package service

import (
	"crynux_relay/models"
	"testing"
)

func TestTaskTypeTag(t *testing.T) {
	if got := taskTypeTag(models.TaskTypeLLM); got != "LLM" {
		t.Fatalf("expected LLM, got %s", got)
	}
	if got := taskTypeTag(models.TaskTypeSD); got != "SD" {
		t.Fatalf("expected SD, got %s", got)
	}
	if got := taskTypeTag(models.TaskTypeSDFTLora); got != "SDFTLora" {
		t.Fatalf("expected SDFTLora, got %s", got)
	}
	if got := taskTypeTag(models.TaskTypeVideo); got != "Video" {
		t.Fatalf("expected Video, got %s", got)
	}
}

func TestShouldLogHealthBoost(t *testing.T) {
	if shouldLogHealthBoost(nodeHealthMetrics{HealthBefore: 1.0, HealthAfter: 1.0}) {
		t.Fatalf("health boost should be skipped when health does not change")
	}
	if !shouldLogHealthBoost(nodeHealthMetrics{HealthBefore: 0.9, HealthAfter: 1.0}) {
		t.Fatalf("health boost should be logged when health changes")
	}
}
