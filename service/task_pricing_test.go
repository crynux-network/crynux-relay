package service

import (
	"crynux_relay/models"
	"database/sql"
	"math"
	"math/big"
	"testing"
	"time"
)

func TestComputeSDPricingUnitsDefaults(t *testing.T) {
	units, err := computeSDPricingUnits(`{}`)
	if err != nil {
		t.Fatalf("compute sd pricing units: %v", err)
	}
	if units != 6 {
		t.Fatalf("expected default sd units 6, got %f", units)
	}
}

func TestComputeSDPricingUnitsExplicitValues(t *testing.T) {
	units, err := computeSDPricingUnits(`{"task_config":{"num_images":2,"image_width":1024,"image_height":1024}}`)
	if err != nil {
		t.Fatalf("compute sd pricing units: %v", err)
	}
	if units != 8 {
		t.Fatalf("expected sd units 8, got %f", units)
	}
}

func TestComputeSDPricingUnitsPartialConfigUsesDefaults(t *testing.T) {
	units, err := computeSDPricingUnits(`{"task_config":{"num_images":1}}`)
	if err != nil {
		t.Fatalf("compute sd pricing units: %v", err)
	}
	if units != 1 {
		t.Fatalf("expected sd units 1, got %f", units)
	}
}

func TestComputeLLMPricingUnits(t *testing.T) {
	initServiceTestConfig(t)

	units, err := computeLLMPricingUnits(`{"generation_config":{"max_new_tokens":512}}`)
	if err != nil {
		t.Fatalf("compute llm pricing units: %v", err)
	}
	if units != 512 {
		t.Fatalf("expected llm units 512, got %f", units)
	}

	units, err = computeLLMPricingUnits(`{"generation_config":{"max_new_tokens":null}}`)
	if err != nil {
		t.Fatalf("compute llm pricing units: %v", err)
	}
	if units != 256 {
		t.Fatalf("expected configured default llm units 256, got %f", units)
	}

	units, err = computeLLMPricingUnits(`{}`)
	if err != nil {
		t.Fatalf("compute llm pricing units: %v", err)
	}
	if units != 256 {
		t.Fatalf("expected configured default llm units 256, got %f", units)
	}
}

func TestComputeEstimatedNodeSecondsLowerBound(t *testing.T) {
	initServiceTestConfig(t)
	InitTaskPricing()

	task := &models.InferenceTask{TaskType: models.TaskTypeSDFTLora, Timeout: 0}
	if got := computeEstimatedNodeSeconds(task, 0); got != minEstimatedNodeSeconds {
		t.Fatalf("expected lower bound %f, got %f", minEstimatedNodeSeconds, got)
	}
}

func TestComputeEstimatedNodeSecondsSDFTLoraUsesTimeout(t *testing.T) {
	initServiceTestConfig(t)
	InitTaskPricing()

	task := &models.InferenceTask{TaskType: models.TaskTypeSDFTLora, Timeout: 3600}
	if got := computeEstimatedNodeSeconds(task, 0); got != 3600 {
		t.Fatalf("expected 3600 seconds, got %f", got)
	}
}

func TestComputeTaskVRAMWeight(t *testing.T) {
	initServiceTestConfig(t)

	// base_vram is 8 in the test config.
	task := &models.InferenceTask{MinVRAM: 24}
	if got := computeTaskVRAMWeight(task); got != 3 {
		t.Fatalf("expected vram weight 3, got %f", got)
	}

	task = &models.InferenceTask{MinVRAM: 4}
	if got := computeTaskVRAMWeight(task); got != 1 {
		t.Fatalf("expected clamped vram weight 1, got %f", got)
	}

	task = &models.InferenceTask{MinVRAM: 4, RequiredGPU: "A100", RequiredGPUVRAM: 40}
	if got := computeTaskVRAMWeight(task); got != 5 {
		t.Fatalf("expected required gpu vram weight 5, got %f", got)
	}
}

func TestApplyTaskPricingComputesPriority(t *testing.T) {
	initServiceTestConfig(t)
	InitTaskPricing()

	fee, ok := new(big.Int).SetString("1000000000000000000", 10)
	if !ok {
		t.Fatal("failed to parse fee")
	}
	task := &models.InferenceTask{
		TaskType: models.TaskTypeSD,
		TaskArgs: `{"task_config":{"num_images":6,"image_width":512,"image_height":512}}`,
		MinVRAM:  16,
		TaskFee:  models.BigInt{Int: *fee},
	}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply task pricing: %v", err)
	}

	if task.PricingUnits != 6 {
		t.Fatalf("expected pricing units 6, got %f", task.PricingUnits)
	}
	// overhead 30 + 6 units * 10 initial seconds per unit = 90 seconds.
	if task.EstimatedNodeSeconds != 90 {
		t.Fatalf("expected estimated node seconds 90, got %f", task.EstimatedNodeSeconds)
	}
	if task.VRAMWeight != 2 {
		t.Fatalf("expected vram weight 2, got %f", task.VRAMWeight)
	}

	expected := new(big.Int).Div(fee, big.NewInt(180))
	if task.Priority.Int.Cmp(expected) != 0 {
		t.Fatalf("expected priority %s, got %s", expected.String(), task.Priority.String())
	}
}

func TestUpdateTaskPricingCalibrationEWMA(t *testing.T) {
	initServiceTestConfig(t)
	InitTaskPricing()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := &models.InferenceTask{
		TaskType:       models.TaskTypeSD,
		PricingUnits:   6,
		StartTime:      sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime: sql.NullTime{Time: start.Add(90 * time.Second), Valid: true},
	}
	UpdateTaskPricingCalibration(task)

	// measured unit seconds = (90 - 30) / 6 = 10; alpha 0.1 over initial 10.
	if got := getCalibratedUnitSeconds(models.TaskTypeSD); math.Abs(got-10) > 1e-9 {
		t.Fatalf("expected calibrated sd unit seconds 10, got %f", got)
	}

	task.ScoreReadyTime = sql.NullTime{Time: start.Add(150 * time.Second), Valid: true}
	UpdateTaskPricingCalibration(task)

	// measured unit seconds = (150 - 30) / 6 = 20; 0.1*20 + 0.9*10 = 11.
	if got := getCalibratedUnitSeconds(models.TaskTypeSD); math.Abs(got-11) > 1e-9 {
		t.Fatalf("expected calibrated sd unit seconds 11, got %f", got)
	}
}

func TestUpdateTaskPricingCalibrationSkipsExecutionBelowOverhead(t *testing.T) {
	initServiceTestConfig(t)
	InitTaskPricing()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := &models.InferenceTask{
		TaskType:       models.TaskTypeSD,
		PricingUnits:   6,
		StartTime:      sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime: sql.NullTime{Time: start.Add(10 * time.Second), Valid: true},
	}
	UpdateTaskPricingCalibration(task)

	// measured unit seconds clamps to 0; 0.1*0 + 0.9*10 = 9.
	if got := getCalibratedUnitSeconds(models.TaskTypeSD); math.Abs(got-9) > 1e-9 {
		t.Fatalf("expected calibrated sd unit seconds 9, got %f", got)
	}
}
