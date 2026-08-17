package service

import (
	"bytes"
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func initTaskPricingTestStore(t *testing.T) {
	t.Helper()
	initServiceTestConfig(t)
	if err := config.GetDB().AutoMigrate(&models.GPUExecutionCalibration{}); err != nil {
		t.Fatalf("migrate gpu calibrations: %v", err)
	}
	if err := InitTaskPricing(context.Background(), config.GetDB()); err != nil {
		t.Fatalf("init task pricing: %v", err)
	}
}

func testSDRate(gpuName string, gpuVram uint64) float64 {
	globalTaskPricing.mu.RLock()
	defer globalTaskPricing.mu.RUnlock()
	for _, cached := range globalTaskPricing.records {
		if cached.record.TaskType == models.TaskTypeSD && cached.record.GPUName == gpuName && cached.record.GPUVram == gpuVram {
			return cached.record.SecondsPerSDPixelStep
		}
	}
	return 0
}

func setTestLLMWorkload(task *models.InferenceTask, textInputBytes *uint64) {
	zero := uint64(0)
	task.LLMTextInputBytes = textInputBytes
	task.LLMImageCount = &zero
	task.LLMImagePixels = &zero
}

func calibrateTestLLMSample(t *testing.T, taskID string, inputBytes, completionTokens uint64, seconds float64) models.GPUExecutionCalibration {
	t.Helper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := &models.InferenceTask{
		TaskIDCommitment: taskID,
		TaskType:         models.TaskTypeLLM,
		Status:           models.TaskEndSuccess,
		LLMInputBytes:    &inputBytes,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(time.Duration(seconds * float64(time.Second))), Valid: true},
	}
	setTestLLMWorkload(task, &inputBytes)
	CaptureTaskExecutionGPUSnapshot(taskID, "A100", 40)
	if err := CalibrateUploadedLLMTask(task, completionTokens); err != nil {
		t.Fatalf("calibrate LLM sample: %v", err)
	}
	globalTaskPricing.mu.RLock()
	defer globalTaskPricing.mu.RUnlock()
	for _, cached := range globalTaskPricing.records {
		if cached.record.TaskType == models.TaskTypeLLM && cached.record.GPUName == "A100" && cached.record.GPUVram == 40 {
			return cached.record
		}
	}
	t.Fatal("calibration record not found")
	return models.GPUExecutionCalibration{}
}

func TestComputeSDPricingUnitsDefaults(t *testing.T) {
	units, err := computeSDPricingUnits(`{}`)
	if err != nil {
		t.Fatalf("compute sd pricing units: %v", err)
	}
	if units != 6*512*512 {
		t.Fatalf("expected default pixel-step units, got %d", units)
	}
}

func TestComputeSDPricingUnitsExplicitValues(t *testing.T) {
	units, err := computeSDPricingUnits(`{"task_config":{"num_images":2,"image_width":1024,"image_height":1024,"steps":20}}`)
	if err != nil {
		t.Fatalf("compute sd pricing units: %v", err)
	}
	if units != 2*1024*1024*20 {
		t.Fatalf("expected exact pixel-step units, got %d", units)
	}
}

func TestComputeSDPricingUnitsPartialConfigUsesDefaults(t *testing.T) {
	units, err := computeSDPricingUnits(`{"task_config":{"num_images":1}}`)
	if err != nil {
		t.Fatalf("compute sd pricing units: %v", err)
	}
	if units != 512*512 {
		t.Fatalf("expected default dimensions and steps, got %d", units)
	}
}

func TestComputeLLMPricingUnits(t *testing.T) {
	initTaskPricingTestStore(t)

	units, err := computeLLMMaxNewTokens(`{"generation_config":{"max_new_tokens":512}}`)
	if err != nil {
		t.Fatalf("compute llm pricing units: %v", err)
	}
	if units != 512 {
		t.Fatalf("expected llm units 512, got %d", units)
	}

	units, err = computeLLMMaxNewTokens(`{"generation_config":{"max_new_tokens":null}}`)
	if err != nil {
		t.Fatalf("compute llm pricing units: %v", err)
	}
	if units != 256 {
		t.Fatalf("expected configured default llm units 256, got %d", units)
	}

	units, err = computeLLMMaxNewTokens(`{}`)
	if err != nil {
		t.Fatalf("compute llm pricing units: %v", err)
	}
	if units != 256 {
		t.Fatalf("expected configured default llm units 256, got %d", units)
	}
}

func TestComputeLLMInputBytesCanonical(t *testing.T) {
	first := `{"model":"ignored","messages":[{"content":"hello","role":"user"}],"tools":[{"function":{"name":"f","parameters":{"b":2,"a":1}},"type":"function"}],"template_args":{"z":2,"a":1},"generation_config":{"max_new_tokens":10}}`
	second := `{
		"generation_config":{"max_new_tokens":999},
		"template_args":{"a":1,"z":2},
		"tools":[{"type":"function","function":{"parameters":{"a":1,"b":2},"name":"f"}}],
		"messages":[{"role":"user","content":"hello"}],
		"model":"different"
	}`
	firstBytes, err := computeLLMInputBytes(first)
	if err != nil {
		t.Fatalf("compute first input bytes: %v", err)
	}
	secondBytes, err := computeLLMInputBytes(second)
	if err != nil {
		t.Fatalf("compute second input bytes: %v", err)
	}
	if firstBytes != secondBytes {
		t.Fatalf("canonical input bytes differ: %d != %d", firstBytes, secondBytes)
	}
}

func TestComputeLLMWorkloadExtractsImageDimensionsAndExcludesPayload(t *testing.T) {
	encodeImage := func(fill color.Color) string {
		t.Helper()
		img := image.NewRGBA(image.Rect(0, 0, 2, 3))
		for y := 0; y < 3; y++ {
			for x := 0; x < 2; x++ {
				img.Set(x, y, fill)
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, img); err != nil {
			t.Fatalf("encode png: %v", err)
		}
		return base64.StdEncoding.EncodeToString(buffer.Bytes())
	}
	taskArgs := func(payload string) string {
		return fmt.Sprintf(`{"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image","base64":%q}]}],"tools":[],"template_args":{}}`, payload)
	}
	first, err := computeLLMWorkload(taskArgs(encodeImage(color.Black)))
	if err != nil {
		t.Fatalf("compute first workload: %v", err)
	}
	second, err := computeLLMWorkload(taskArgs(encodeImage(color.White)))
	if err != nil {
		t.Fatalf("compute second workload: %v", err)
	}
	if first.imageCount != 1 || first.imagePixels != 6 {
		t.Fatalf("unexpected image workload: %+v", first)
	}
	if first.textInputBytes != second.textInputBytes {
		t.Fatalf("base64 payload changed text bytes: %d != %d", first.textInputBytes, second.textInputBytes)
	}
}

func TestComputeLLMWorkloadRejectsUnreadableImage(t *testing.T) {
	_, err := computeLLMWorkload(`{"messages":[{"role":"user","content":[{"type":"image","base64":"bm90LWEtc3VwcG9ydGVkLWltYWdl"}]}]}`)
	if err == nil {
		t.Fatal("expected unreadable image error")
	}
}

func TestComputeEstimatedNodeSecondsLowerBound(t *testing.T) {
	initTaskPricingTestStore(t)
	task := &models.InferenceTask{TaskType: models.TaskTypeSDFTLora, Timeout: 0}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply task pricing: %v", err)
	}
	if task.EstimatedNodeSeconds != minEstimatedNodeSeconds {
		t.Fatalf("expected lower bound %f, got %f", minEstimatedNodeSeconds, task.EstimatedNodeSeconds)
	}
}

func TestComputeEstimatedNodeSecondsSDFTLoraUsesTimeout(t *testing.T) {
	initTaskPricingTestStore(t)

	task := &models.InferenceTask{TaskType: models.TaskTypeSDFTLora, Timeout: 3600}
	got, err := computeEstimatedNodeSeconds(task, executionParameters{})
	if err != nil {
		t.Fatalf("compute estimated seconds: %v", err)
	}
	if got != 3600 {
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
	initTaskPricingTestStore(t)

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

	if task.SDUnits == nil || *task.SDUnits != 6*512*512 {
		t.Fatalf("expected exact pixel-step units")
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

func TestApplyLLMTaskPricingUsesInputAndOutputWork(t *testing.T) {
	initTaskPricingTestStore(t)
	fee := big.NewInt(1_000_000_000)
	task := &models.InferenceTask{
		TaskType: models.TaskTypeLLM,
		TaskArgs: `{"messages":[{"role":"user","content":"hello"}],"tools":[],"template_args":{},"generation_config":{"max_new_tokens":512}}`,
		MinVRAM:  16,
		TaskFee:  models.BigInt{Int: *fee},
	}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply task pricing: %v", err)
	}
	if task.LLMInputBytes == nil {
		t.Fatal("expected llm_input_bytes")
	}
	if task.LLMMaxNewTokens == nil || *task.LLMMaxNewTokens != 512 {
		t.Fatalf("expected llm_max_new_tokens 512, got %+v", task.LLMMaxNewTokens)
	}
	expected := 30 + float64(*task.LLMInputBytes)*0.0001 + 512*0.1
	if math.Abs(task.EstimatedNodeSeconds-expected) > 1e-9 {
		t.Fatalf("expected %f seconds, got %f", expected, task.EstimatedNodeSeconds)
	}
}

func TestComputeEstimatedNodeSecondsUsesStoredLLMMaxNewTokens(t *testing.T) {
	initTaskPricingTestStore(t)
	input := uint64(100)
	tokens := uint64(512)
	task := &models.InferenceTask{
		TaskType:        models.TaskTypeLLM,
		TaskArgs:        `{"generation_config":{"max_new_tokens":1}}`,
		LLMInputBytes:   &input,
		LLMMaxNewTokens: &tokens,
	}
	zero := uint64(0)
	task.LLMTextInputBytes = &input
	task.LLMImageCount = &zero
	task.LLMImagePixels = &zero
	got, err := computeEstimatedNodeSeconds(task, executionParameters{llm: llmExecutionParameters{
		constantSeconds: 30, secondsPerInputByte: 0.0001, secondsPerOutputToken: 0.1,
	}})
	if err != nil {
		t.Fatalf("compute estimated seconds: %v", err)
	}
	expected := 30 + 100*0.0001 + 512*0.1
	if math.Abs(got-expected) > 1e-9 {
		t.Fatalf("expected stored max_new_tokens estimate %f, got %f", expected, got)
	}
}

func TestCalibrateUploadedLLMTaskRequiresTerminalSuccessStatus(t *testing.T) {
	initTaskPricingTestStore(t)
	key := gpuCalibrationKey{taskType: models.TaskTypeLLM, name: "A100", vram: 40, executionDType: models.AutoExecutionDType}
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[key] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		TaskType:                 models.TaskTypeLLM,
		GPUName:                  "A100",
		GPUVram:                  40,
		ExecutionDType:           models.AutoExecutionDType,
		LLMConstantSeconds:       30,
		LLMSecondsPerInputByte:   0.0001,
		LLMSecondsPerOutputToken: 0.1,
	}}
	globalTaskPricing.mu.Unlock()

	input := uint64(100)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := &models.InferenceTask{
		TaskIDCommitment: "llm-status-gate",
		TaskType:         models.TaskTypeLLM,
		Status:           models.TaskValidated,
		LLMInputBytes:    &input,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(20 * time.Second), Valid: true},
	}
	setTestLLMWorkload(task, &input)
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateUploadedLLMTask(task, 10); err != nil {
		t.Fatalf("calibrate with non-terminal status: %v", err)
	}
	globalTaskPricing.mu.RLock()
	samplesBefore := globalTaskPricing.records[key].record.LLMSuccessSamples
	globalTaskPricing.mu.RUnlock()
	if samplesBefore != 0 {
		t.Fatalf("expected no calibration before EndSuccess, got %d samples", samplesBefore)
	}

	task.Status = models.TaskEndSuccess
	if err := CalibrateUploadedLLMTask(task, 10); err != nil {
		t.Fatalf("calibrate after EndSuccess: %v", err)
	}
	globalTaskPricing.mu.RLock()
	samplesAfter := globalTaskPricing.records[key].record.LLMSuccessSamples
	globalTaskPricing.mu.RUnlock()
	if samplesAfter != 1 {
		t.Fatalf("expected calibration after EndSuccess memory status, got %d samples", samplesAfter)
	}
}

func TestUpdateTaskPricingCalibrationEWMA(t *testing.T) {
	initTaskPricingTestStore(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	units := uint64(6 * 512 * 512)
	task := &models.InferenceTask{
		TaskIDCommitment: "task-1",
		TaskType:         models.TaskTypeSD,
		Status:           models.TaskValidated,
		SDUnits:          &units,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(90 * time.Second), Valid: true},
	}
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateValidatedSDTask(task); err != nil {
		t.Fatalf("calibrate sd task: %v", err)
	}

	if got := testSDRate("A100", 40) * 512 * 512; math.Abs(got-10) > 1e-9 {
		t.Fatalf("expected calibrated sd unit seconds 10, got %f", got)
	}

	task.ScoreReadyTime = sql.NullTime{Time: start.Add(150 * time.Second), Valid: true}
	if err := CalibrateValidatedSDTask(task); err != nil {
		t.Fatalf("calibrate sd task: %v", err)
	}

	if got := testSDRate("A100", 40) * 512 * 512; math.Abs(got-11) > 1e-9 {
		t.Fatalf("expected calibrated sd unit seconds 11, got %f", got)
	}
}

func TestUpdateTaskPricingCalibrationSkipsExecutionBelowOverhead(t *testing.T) {
	initTaskPricingTestStore(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	units := uint64(6 * 512 * 512)
	task := &models.InferenceTask{
		TaskIDCommitment: "task-2",
		TaskType:         models.TaskTypeSD,
		Status:           models.TaskValidated,
		SDUnits:          &units,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(10 * time.Second), Valid: true},
	}
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateValidatedSDTask(task); err != nil {
		t.Fatalf("calibrate sd task: %v", err)
	}

	if got := testSDRate("A100", 40) * 512 * 512; math.Abs(got-9) > 1e-9 {
		t.Fatalf("expected calibrated sd unit seconds 9, got %f", got)
	}
}

func TestTaskPricingAggregationUsesSimpleGPUAverage(t *testing.T) {
	initTaskPricingTestStore(t)
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[gpuCalibrationKey{name: "A", vram: 40}] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		GPUName: "A", GPUVram: 40, SecondsPerSDPixelStep: 10.0 / (512 * 512), SDSuccessSamples: 100,
	}}
	globalTaskPricing.records[gpuCalibrationKey{name: "B", vram: 80}] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		GPUName: "B", GPUVram: 80, SecondsPerSDPixelStep: 30.0 / (512 * 512), SDSuccessSamples: 1,
	}}
	globalTaskPricing.mu.Unlock()
	task := &models.InferenceTask{
		TaskType: models.TaskTypeSD,
		TaskArgs: `{"task_config":{"num_images":1,"image_width":512,"image_height":512,"steps":1}}`,
		MinVRAM:  32,
	}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply task pricing: %v", err)
	}
	if math.Abs(task.EstimatedNodeSeconds-50) > 1e-9 {
		t.Fatalf("expected simple average estimate 50, got %f", task.EstimatedNodeSeconds)
	}
}

func TestRequiredGPUUsesExactParametersWithoutAggregateKey(t *testing.T) {
	initTaskPricingTestStore(t)
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[gpuCalibrationKey{name: "B", vram: 40}] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		GPUName: "B", GPUVram: 40, SecondsPerSDPixelStep: 20.0 / (512 * 512), SDSuccessSamples: 1,
	}}
	globalTaskPricing.mu.Unlock()
	task := &models.InferenceTask{
		TaskType:        models.TaskTypeSD,
		TaskArgs:        `{"task_config":{"num_images":1,"image_width":512,"image_height":512,"steps":1}}`,
		RequiredGPU:     "A",
		RequiredGPUVRAM: 40,
	}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply task pricing: %v", err)
	}
	if math.Abs(task.EstimatedNodeSeconds-50) > 1e-9 {
		t.Fatalf("expected same-VRAM initialization, got %f", task.EstimatedNodeSeconds)
	}
	globalTaskPricing.mu.RLock()
	defer globalTaskPricing.mu.RUnlock()
	if len(globalTaskPricing.aggregates) != 0 {
		t.Fatal("required GPU task created an aggregate pricing key")
	}
}

func TestComputeExecutionTimeoutColdStartUsesSlowestReadySameVRAM(t *testing.T) {
	initTaskPricingTestStore(t)
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[gpuCalibrationKey{name: "target", vram: 40}] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		GPUName: "target", GPUVram: 40, SecondsPerSDPixelStep: 1.0 / (512 * 512), SDSuccessSamples: 2,
	}}
	globalTaskPricing.records[gpuCalibrationKey{name: "slow", vram: 40}] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		GPUName: "slow", GPUVram: 40, SecondsPerSDPixelStep: 100.0 / (512 * 512), SDSuccessSamples: 10,
	}}
	globalTaskPricing.mu.Unlock()
	units := uint64(512 * 512)
	task := &models.InferenceTask{TaskType: models.TaskTypeSD, SDUnits: &units}
	timeout, err := ComputeExecutionTimeout(task, "target", 40)
	if err != nil {
		t.Fatalf("compute timeout: %v", err)
	}
	if timeout != 260 {
		t.Fatalf("expected cold-start timeout 260, got %d", timeout)
	}

	description, err := DescribeExecutionTimeout(task, "target", 40)
	if err != nil {
		t.Fatalf("describe timeout: %v", err)
	}
	if !description.ColdStart {
		t.Fatal("expected cold-start description")
	}
	if description.ComputedTimeout != 260 {
		t.Fatalf("expected described timeout 260, got %d", description.ComputedTimeout)
	}
	if description.PredictedExecutionSeconds <= 0 {
		t.Fatalf("expected positive predicted seconds, got %v", description.PredictedExecutionSeconds)
	}
	if description.TimeoutMultiplier <= 0 || description.MinExecutionTimeoutSeconds == 0 || description.MaxExecutionTimeoutSeconds == 0 {
		t.Fatalf("expected timeout clamp inputs, got %+v", description)
	}
}

func TestComputeExecutionTimeoutUsesReadyCalibrationBelowInitialPrediction(t *testing.T) {
	initTaskPricingTestStore(t)
	units := uint64(512 * 512)
	rate := 1.0 / float64(units)
	record := models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: "target", GPUVram: 40,
		SecondsPerSDPixelStep: rate, SDSuccessSamples: 10,
	}
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[keyFromRecord(record)] = &cachedGPUCalibration{record: record}
	globalTaskPricing.mu.Unlock()

	task := &models.InferenceTask{TaskType: models.TaskTypeSD, SDUnits: &units}
	description, err := DescribeExecutionTimeout(task, "target", 40)
	if err != nil {
		t.Fatalf("describe timeout: %v", err)
	}
	expected := config.GetConfig().TaskPricing.OverheadSeconds + 1
	if math.Abs(description.PredictedExecutionSeconds-expected) > 1e-9 {
		t.Fatalf("predicted seconds = %f, want ready calibration %f", description.PredictedExecutionSeconds, expected)
	}
	if description.ColdStart {
		t.Fatal("ready exact calibration must not be reported as cold start")
	}
}

func TestComputeExecutionTimeoutIncludesModelSwitch(t *testing.T) {
	initTaskPricingTestStore(t)
	textBytes, maxTokens, imageCount, imagePixels := uint64(100), uint64(10), uint64(0), uint64(0)
	task := &models.InferenceTask{
		TaskType: models.TaskTypeLLM, LLMTextInputBytes: &textBytes, LLMMaxNewTokens: &maxTokens,
		LLMImageCount: &imageCount, LLMImagePixels: &imagePixels,
	}
	withoutSwitch, err := ComputeExecutionTimeout(task, "A100", 40, false)
	if err != nil {
		t.Fatalf("compute timeout without switch: %v", err)
	}
	withSwitch, err := ComputeExecutionTimeout(task, "A100", 40, true)
	if err != nil {
		t.Fatalf("compute timeout with switch: %v", err)
	}
	expectedIncrease := uint64(math.Ceil(config.GetConfig().TaskPricing.InitialLLMModelSwitchSeconds *
		config.GetConfig().TaskPricing.TimeoutMultiplier))
	if withSwitch-withoutSwitch != expectedIncrease {
		t.Fatalf("model switch timeout increase = %d, want %d", withSwitch-withoutSwitch, expectedIncrease)
	}
}

func TestLLMEWLSSingleLargeInputSampleFits(t *testing.T) {
	for _, inputBytes := range []uint64{12_221, 18_271} {
		t.Run(fmt.Sprintf("%d_bytes", inputBytes), func(t *testing.T) {
			initTaskPricingTestStore(t)
			record := calibrateTestLLMSample(t, fmt.Sprintf("large-input-%d", inputBytes), inputBytes, 128, 60)

			if record.LLMSuccessSamples != 1 {
				t.Fatalf("LLMSuccessSamples = %d, want 1", record.LLMSuccessSamples)
			}
			for i, coefficient := range parametersFromRecord(&record).llm.values() {
				if coefficient < 0 || math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
					t.Fatalf("coefficient %d is not finite and non-negative: %v", i, coefficient)
				}
			}
			initial := initialExecutionParameters().llm
			if math.Abs(record.LLMModelSwitchSeconds-initial.modelSwitchSeconds) > 1e-9 ||
				math.Abs(record.LLMSecondsPerImage-initial.secondsPerImage) > 1e-9 ||
				math.Abs(record.LLMSecondsPerMegapixel-initial.secondsPerMegapixel) > 1e-9 {
				t.Fatalf("unobserved coefficients did not retain their initial values: %+v", record)
			}
		})
	}
}

func TestLLMEWLSStoresSmallInputInOriginalByteUnits(t *testing.T) {
	initTaskPricingTestStore(t)
	record := calibrateTestLLMSample(t, "small-input", 10, 20, 20)
	alpha := config.GetConfig().TaskPricing.CalibrationAlpha

	if record.LLMXTX01 != alpha*10 || record.LLMXTX11 != alpha*10*10 {
		t.Fatalf("input-byte matrix entries were scaled before persistence: XTX01=%v XTX11=%v", record.LLMXTX01, record.LLMXTX11)
	}
	if record.LLMXTY1 != alpha*10*20 {
		t.Fatalf("input-byte XTy entry = %v, want %v", record.LLMXTY1, alpha*10*20)
	}
}

func TestLLMEWLSFitsIndependentNonnegativeCoefficients(t *testing.T) {
	initTaskPricingTestStore(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	samples := []struct {
		input, output uint64
		seconds       float64
	}{
		{100, 10, 20},
		{200, 40, 40},
		{400, 20, 40},
	}
	for i, sample := range samples {
		taskID := fmt.Sprintf("llm-%d", i)
		task := &models.InferenceTask{
			TaskIDCommitment: taskID,
			TaskType:         models.TaskTypeLLM,
			Status:           models.TaskEndSuccess,
			LLMInputBytes:    &sample.input,
			StartTime:        sql.NullTime{Time: start, Valid: true},
			ScoreReadyTime:   sql.NullTime{Time: start.Add(time.Duration(sample.seconds * float64(time.Second))), Valid: true},
		}
		setTestLLMWorkload(task, &sample.input)
		CaptureTaskExecutionGPUSnapshot(taskID, "A100", 40)
		if err := CalibrateUploadedLLMTask(task, sample.output); err != nil {
			t.Fatalf("calibrate sample %d: %v", i, err)
		}
	}
	globalTaskPricing.mu.RLock()
	var record models.GPUExecutionCalibration
	for _, cached := range globalTaskPricing.records {
		if cached.record.TaskType == models.TaskTypeLLM && cached.record.GPUName == "A100" && cached.record.GPUVram == 40 {
			record = cached.record
		}
	}
	globalTaskPricing.mu.RUnlock()
	coefficients := []float64{record.LLMConstantSeconds, record.LLMSecondsPerInputByte, record.LLMSecondsPerOutputToken}
	for i, coefficient := range coefficients {
		if coefficient < 0 {
			t.Fatalf("coefficient %d is negative: %f", i, coefficient)
		}
	}
	if math.Abs(record.LLMModelSwitchSeconds-config.GetConfig().TaskPricing.InitialLLMModelSwitchSeconds) > 1e-9 ||
		math.Abs(record.LLMSecondsPerImage-config.GetConfig().TaskPricing.InitialLLMSecondsPerImage) > 1e-9 ||
		math.Abs(record.LLMSecondsPerMegapixel-config.GetConfig().TaskPricing.InitialLLMSecondsPerMegapixel) > 1e-9 {
		t.Fatalf("unobserved coefficients did not retain their initial values: %+v", record)
	}
}

func TestCalibrateUploadedLLMTaskDoesNotCommitWhenFitFails(t *testing.T) {
	initTaskPricingTestStore(t)
	cfg := config.GetConfig()
	originalRegularization := cfg.TaskPricing.CalibrationRegularization
	cfg.TaskPricing.CalibrationRegularization = 0
	t.Cleanup(func() {
		cfg.TaskPricing.CalibrationRegularization = originalRegularization
	})

	key := gpuCalibrationKey{taskType: models.TaskTypeLLM, name: "A100", vram: 40, executionDType: models.AutoExecutionDType}
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[key] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		TaskType:                 models.TaskTypeLLM,
		GPUName:                  "A100",
		GPUVram:                  40,
		ExecutionDType:           models.AutoExecutionDType,
		LLMConstantSeconds:       30,
		LLMSecondsPerInputByte:   0.0001,
		LLMSecondsPerOutputToken: 0.1,
		LLMSuccessSamples:        4,
	}}
	globalTaskPricing.mu.Unlock()

	input := uint64(100)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := &models.InferenceTask{
		TaskIDCommitment: "llm-fit-fail",
		TaskType:         models.TaskTypeLLM,
		Status:           models.TaskEndSuccess,
		LLMInputBytes:    &input,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(20 * time.Second), Valid: true},
	}
	setTestLLMWorkload(task, &input)
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateUploadedLLMTask(task, 10); err == nil {
		t.Fatal("expected fit failure")
	} else if !strings.Contains(err.Error(), "does not produce a positive finite scale") {
		t.Fatalf("unexpected fit failure: %v", err)
	}

	globalTaskPricing.mu.RLock()
	cached := globalTaskPricing.records[key]
	record := cached.record
	dirty := cached.dirtyVersion != 0
	globalTaskPricing.mu.RUnlock()
	if record.LLMSuccessSamples != 4 {
		t.Fatalf("LLMSuccessSamples = %d, want 4", record.LLMSuccessSamples)
	}
	if record.LLMConstantSeconds != 30 || record.LLMSecondsPerInputByte != 0.0001 || record.LLMSecondsPerOutputToken != 0.1 {
		t.Fatalf("coefficients changed after failed fit: %+v", record)
	}
	if record.LLMXTX00 != 0 || record.LLMXTX11 != 0 || record.LLMXTX22 != 0 ||
		record.LLMXTY0 != 0 || record.LLMXTY1 != 0 || record.LLMXTY2 != 0 {
		t.Fatalf("matrix changed after failed fit: %+v", record)
	}
	if dirty {
		t.Fatal("failed fit marked record dirty")
	}
}

func TestDeleteGroupRefundExecutionGPUSnapshots(t *testing.T) {
	initTaskPricingTestStore(t)
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("migrate inference tasks: %v", err)
	}

	const taskID = "cleanup-group"
	validated := &models.InferenceTask{
		TaskIDCommitment: "cleanup-validated",
		TaskID:           taskID,
		Status:           models.TaskGroupValidated,
		TaskType:         models.TaskTypeLLM,
	}
	refund := &models.InferenceTask{
		TaskIDCommitment: "cleanup-refund",
		TaskID:           taskID,
		Status:           models.TaskEndGroupRefund,
		TaskType:         models.TaskTypeLLM,
	}
	other := &models.InferenceTask{
		TaskIDCommitment: "cleanup-other",
		TaskID:           taskID,
		Status:           models.TaskEndAborted,
		TaskType:         models.TaskTypeLLM,
	}
	for _, task := range []*models.InferenceTask{validated, refund, other} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("create task %s: %v", task.TaskIDCommitment, err)
		}
		CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	}

	DeleteGroupRefundExecutionGPUSnapshots(ctx, db, taskID)

	if _, _, ok := TaskExecutionGPUSnapshot(refund.TaskIDCommitment); ok {
		t.Fatal("expected EndGroupRefund snapshot deleted")
	}
	if _, _, ok := TaskExecutionGPUSnapshot(validated.TaskIDCommitment); !ok {
		t.Fatal("expected GroupValidated snapshot retained by this helper")
	}
	if _, _, ok := TaskExecutionGPUSnapshot(other.TaskIDCommitment); !ok {
		t.Fatal("expected non-refund snapshot retained by this helper")
	}
}

func TestCalibrationFlushPersistsDirtyRecord(t *testing.T) {
	initTaskPricingTestStore(t)
	start := time.Now()
	units := uint64(512 * 512)
	task := &models.InferenceTask{
		TaskIDCommitment: "flush-task",
		TaskType:         models.TaskTypeSD,
		Status:           models.TaskValidated,
		SDUnits:          &units,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(60 * time.Second), Valid: true},
	}
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateValidatedSDTask(task); err != nil {
		t.Fatalf("calibrate: %v", err)
	}
	if err := FlushTaskPricingCalibration(context.Background(), config.GetDB()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	records, err := models.LoadGPUExecutionCalibrations(context.Background(), config.GetDB())
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 1 || records[0].SDSuccessSamples != 1 {
		t.Fatalf("unexpected persisted records: %+v", records)
	}
}

func TestCalibrationFlushRetainsConcurrentUpdate(t *testing.T) {
	initTaskPricingTestStore(t)
	start := time.Now()
	units := uint64(512 * 512)
	task := &models.InferenceTask{
		TaskIDCommitment: "concurrent-flush", TaskType: models.TaskTypeSD, Status: models.TaskValidated,
		ModelName: "model", RequestedDType: "auto", MinVRAM: 8, SDUnits: &units,
		StartTime:      sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime: sql.NullTime{Time: start.Add(60 * time.Second), Valid: true},
	}
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateValidatedSDTask(task); err != nil {
		t.Fatalf("initial calibration: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	const callback = "test:block_calibration_flush"
	db := config.GetDB()
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(*gorm.DB) {
		close(entered)
		<-release
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	done := make(chan error, 1)
	go func() {
		done <- FlushTaskPricingCalibration(context.Background(), db)
	}()
	<-entered
	task.ScoreReadyTime.Time = start.Add(90 * time.Second)
	if err := CalibrateValidatedSDTask(task); err != nil {
		t.Fatalf("concurrent calibration: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("flush: %v", err)
	}
	key := taskCalibrationKey(task, "A100", 40)
	globalTaskPricing.mu.RLock()
	dirty := globalTaskPricing.records[key].dirtyVersion
	globalTaskPricing.mu.RUnlock()
	if dirty == 0 {
		t.Fatal("concurrent update was incorrectly marked clean")
	}
}

func TestInitTaskPricingResetsOldLLMFormulaAndPreservesSD(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.GPUExecutionCalibration{}); err != nil {
		t.Fatalf("migrate calibrations: %v", err)
	}
	record := models.GPUExecutionCalibration{
		GPUName: "A100", GPUVram: 40, SecondsPerSDPixelStep: 0.25, SDSuccessSamples: 7,
		LLMConstantSeconds: 1, LLMSecondsPerInputByte: 2, LLMSecondsPerOutputToken: 3,
		LLMFormulaVersion: 1, LLMSuccessSamples: 9, LLMXTX00: 1, LLMXTY0: 1,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed old calibration: %v", err)
	}
	if err := InitTaskPricing(context.Background(), db); err != nil {
		t.Fatalf("init task pricing: %v", err)
	}
	globalTaskPricing.mu.RLock()
	cached := globalTaskPricing.records[gpuCalibrationKey{name: "A100", vram: 40}]
	globalTaskPricing.mu.RUnlock()
	if cached.record.SecondsPerSDPixelStep != 0.25 || cached.record.SDSuccessSamples != 7 {
		t.Fatalf("SD calibration changed: %+v", cached.record)
	}
	if cached.record.LLMFormulaVersion != llmFormulaVersion || cached.record.LLMSuccessSamples != 0 ||
		cached.record.LLMConstantSeconds != config.GetConfig().TaskPricing.InitialLLMConstantSeconds ||
		cached.dirtyVersion == 0 {
		t.Fatalf("old LLM formula was not reset: %+v", cached)
	}
}

func TestCleanupTaskExecutionGPUSnapshotsExcludesQueueTimeout(t *testing.T) {
	initTaskPricingTestStore(t)
	cfg := config.GetConfig().TaskPricing
	maxAgeWithoutQueue := time.Duration(cfg.MaxExecutionTimeoutSeconds+
		cfg.AppValidationTimeoutSeconds+cfg.ResultUploadTimeoutSeconds) * time.Second
	now := time.Now()

	CaptureTaskExecutionGPUSnapshot("fresh-snapshot", "A100", 40)
	globalTaskPricing.mu.Lock()
	globalTaskPricing.snapshots["stale-after-start"] = executionGPUSnapshot{
		GPUName: "A100", GPUVram: 40, CapturedAt: now.Add(-(maxAgeWithoutQueue + time.Second)),
	}
	globalTaskPricing.snapshots["still-within-queue-window"] = executionGPUSnapshot{
		GPUName: "A100", GPUVram: 40,
		CapturedAt: now.Add(-(maxAgeWithoutQueue - time.Minute)),
	}
	globalTaskPricing.mu.Unlock()

	removed := CleanupTaskExecutionGPUSnapshots(now)
	if removed != 1 {
		t.Fatalf("expected one stale snapshot removed, got %d", removed)
	}
	if _, _, ok := TaskExecutionGPUSnapshot("stale-after-start"); ok {
		t.Fatal("snapshot older than post-start lifecycle must be removed")
	}
	if _, _, ok := TaskExecutionGPUSnapshot("still-within-queue-window"); !ok {
		t.Fatal("snapshot still inside post-start lifecycle must be retained")
	}
	if _, _, ok := TaskExecutionGPUSnapshot("fresh-snapshot"); !ok {
		t.Fatal("fresh snapshot must be retained")
	}
}

func TestCalibrateUploadedLLMTaskSkipsResidualTruncateWhenPredictionNonPositive(t *testing.T) {
	initTaskPricingTestStore(t)
	key := gpuCalibrationKey{taskType: models.TaskTypeLLM, name: "A100", vram: 40, executionDType: models.AutoExecutionDType}
	globalTaskPricing.mu.Lock()
	globalTaskPricing.records[key] = &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		TaskType: models.TaskTypeLLM, GPUName: "A100", GPUVram: 40, ExecutionDType: models.AutoExecutionDType,
	}}
	globalTaskPricing.mu.Unlock()

	input := uint64(1000)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := &models.InferenceTask{
		TaskIDCommitment: "llm-zero-prediction",
		TaskType:         models.TaskTypeLLM,
		Status:           models.TaskEndSuccess,
		LLMInputBytes:    &input,
		StartTime:        sql.NullTime{Time: start, Valid: true},
		ScoreReadyTime:   sql.NullTime{Time: start.Add(120 * time.Second), Valid: true},
	}
	setTestLLMWorkload(task, &input)
	CaptureTaskExecutionGPUSnapshot(task.TaskIDCommitment, "A100", 40)
	if err := CalibrateUploadedLLMTask(task, 50); err != nil {
		t.Fatalf("calibrate: %v", err)
	}

	globalTaskPricing.mu.RLock()
	record := globalTaskPricing.records[key].record
	globalTaskPricing.mu.RUnlock()
	if record.LLMSuccessSamples != 1 {
		t.Fatalf("expected one success sample, got %d", record.LLMSuccessSamples)
	}
	if record.LLMXTY0 <= 0 {
		t.Fatalf("expected non-truncated actual to update XTy, got xty0=%f", record.LLMXTY0)
	}
	if record.LLMConstantSeconds <= 0 && record.LLMSecondsPerInputByte <= 0 && record.LLMSecondsPerOutputToken <= 0 {
		t.Fatalf("expected at least one positive fitted coefficient, got %+v", record)
	}
}

func TestCalibrationSeparatesModelConfigAndUpdatesVRAMRange(t *testing.T) {
	initTaskPricingTestStore(t)
	start := time.Now()
	units := uint64(512 * 512)
	calibrate := func(id, model, variant, dtype string, quantizeBits, minVRAM uint64) {
		task := &models.InferenceTask{
			TaskIDCommitment: id, TaskType: models.TaskTypeSD, Status: models.TaskValidated,
			ModelName: model, ModelVariant: variant, RequestedDType: "auto", ExecutionDType: dtype,
			QuantizeBits: quantizeBits, MinVRAM: minVRAM, SDUnits: &units,
			StartTime:      sql.NullTime{Time: start, Valid: true},
			ScoreReadyTime: sql.NullTime{Time: start.Add(60 * time.Second), Valid: true},
		}
		CaptureTaskExecutionGPUSnapshot(id, "A100", 40)
		if err := CalibrateValidatedSDTask(task); err != nil {
			t.Fatalf("calibrate %s: %v", id, err)
		}
	}
	calibrate("a-8", "model-a", "fp16", "float16", 0, 8)
	calibrate("a-16", "model-a", "fp16", "float16", 0, 16)
	calibrate("b-12", "model-b", "fp16", "bfloat16", 0, 12)
	calibrate("variant", "model-a", "bf16", "float16", 0, 12)
	calibrate("quantized", "model-a", "fp16", "float16", 4, 12)

	globalTaskPricing.mu.RLock()
	defer globalTaskPricing.mu.RUnlock()
	if len(globalTaskPricing.records) != 4 {
		t.Fatalf("records = %d, want 4", len(globalTaskPricing.records))
	}
	key := gpuCalibrationKey{
		taskType: models.TaskTypeSD, name: "A100", vram: 40, modelName: "model-a",
		modelVariant: "fp16", executionDType: "float16",
	}
	record := globalTaskPricing.records[key].record
	if record.MinVRAMRequirement != 8 || record.MaxVRAMRequirement != 16 || record.SDSuccessSamples != 2 {
		t.Fatalf("unexpected model-a range: %+v", record)
	}
}

func TestAutoPricingAveragesReportedDTypes(t *testing.T) {
	initTaskPricingTestStore(t)
	globalTaskPricing.mu.Lock()
	for _, item := range []struct {
		dtype string
		rate  float64
	}{{"auto", 10}, {"bfloat16", 30}} {
		record := models.GPUExecutionCalibration{
			TaskType: models.TaskTypeSD, GPUName: "A100", GPUVram: 40, ModelName: "model",
			ExecutionDType: item.dtype, MinVRAMRequirement: 8, MaxVRAMRequirement: 24,
			SecondsPerSDPixelStep: item.rate / (512 * 512), SDSuccessSamples: 10,
		}
		globalTaskPricing.records[keyFromRecord(record)] = &cachedGPUCalibration{record: record}
	}
	globalTaskPricing.mu.Unlock()
	task := &models.InferenceTask{
		TaskType: models.TaskTypeSD, TaskArgs: `{"base_model":"model","task_config":{"num_images":1}}`, MinVRAM: 16,
	}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply pricing: %v", err)
	}
	if math.Abs(task.EstimatedNodeSeconds-50) > 1e-9 {
		t.Fatalf("estimated seconds = %v, want 50", task.EstimatedNodeSeconds)
	}
}

func TestUnknownModelUsesNearestVRAMInterval(t *testing.T) {
	initTaskPricingTestStore(t)
	globalTaskPricing.mu.Lock()
	for _, record := range []models.GPUExecutionCalibration{
		{TaskType: models.TaskTypeSD, GPUName: "A100", GPUVram: 40, ModelName: "small", ExecutionDType: "auto", MinVRAMRequirement: 8, MaxVRAMRequirement: 12, SecondsPerSDPixelStep: 100.0 / (512 * 512), SDSuccessSamples: 10},
		{TaskType: models.TaskTypeSD, GPUName: "A100", GPUVram: 40, ModelName: "large", ExecutionDType: "auto", MinVRAMRequirement: 20, MaxVRAMRequirement: 24, SecondsPerSDPixelStep: 20.0 / (512 * 512), SDSuccessSamples: 10},
	} {
		globalTaskPricing.records[keyFromRecord(record)] = &cachedGPUCalibration{record: record}
	}
	globalTaskPricing.mu.Unlock()
	task := &models.InferenceTask{TaskType: models.TaskTypeSD, TaskArgs: `{"base_model":"unknown","task_config":{"num_images":1}}`, MinVRAM: 18}
	if err := ApplyTaskPricing(task); err != nil {
		t.Fatalf("apply pricing: %v", err)
	}
	if math.Abs(task.EstimatedNodeSeconds-50) > 1e-9 {
		t.Fatalf("estimated seconds = %v, want nearest interval estimate 50", task.EstimatedNodeSeconds)
	}
}

func TestUnknownModelTimeoutUsesMaximumAtEqualDistance(t *testing.T) {
	initTaskPricingTestStore(t)
	globalTaskPricing.mu.Lock()
	for _, rate := range []float64{10, 100} {
		record := models.GPUExecutionCalibration{
			TaskType: models.TaskTypeSD, GPUName: "A100", GPUVram: 40,
			ModelName: fmt.Sprintf("model-%v", rate), ExecutionDType: "auto",
			MinVRAMRequirement: 16, MaxVRAMRequirement: 16,
			SecondsPerSDPixelStep: rate / (512 * 512), SDSuccessSamples: 10,
		}
		globalTaskPricing.records[keyFromRecord(record)] = &cachedGPUCalibration{record: record}
	}
	globalTaskPricing.mu.Unlock()
	units := uint64(512 * 512)
	task := &models.InferenceTask{
		TaskType: models.TaskTypeSD, ModelName: "unknown", RequestedDType: "auto",
		MinVRAM: 16, SDUnits: &units,
	}
	description, err := DescribeExecutionTimeout(task, "A100", 40)
	if err != nil {
		t.Fatalf("describe timeout: %v", err)
	}
	if description.ComputedTimeout != 260 || !description.FallbackUsed {
		t.Fatalf("unexpected conservative fallback: %+v", description)
	}
}
