package service

import (
	"crynux_relay/models"
	"math"
	"testing"
)

func putTestCalibration(record models.GPUExecutionCalibration) {
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	globalTaskPricing.records[keyFromRecord(record)] = &cachedGPUCalibration{record: record}
}

func TestGetSDExecutionTimeCoefficientsMinVRAMMatchesPricing(t *testing.T) {
	initTaskPricingTestStore(t)
	model := "stabilityai/stable-diffusion-xl-base-1.0"
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: "A", GPUVram: 24,
		ModelName: model, ExecutionDType: models.AutoExecutionDType,
		SDOverheadSeconds: 30, SDFormulaVersion: sdFormulaVersion, SecondsPerSDPixelStep: 0.1, SDSuccessSamples: 1,
	})
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: "B", GPUVram: 48,
		ModelName: model, ExecutionDType: models.AutoExecutionDType,
		SDOverheadSeconds: 30, SDFormulaVersion: sdFormulaVersion, SecondsPerSDPixelStep: 0.3, SDSuccessSamples: 1,
	})

	query := TaskExecutionTimeQuery{ModelName: model, MinVRAM: 24}
	got := GetSDExecutionTimeCoefficients(query)
	want := getTaskPricingParameters(&models.InferenceTask{
		TaskType: models.TaskTypeSD, ModelName: model, RequestedDType: models.AutoExecutionDType, MinVRAM: 24,
	})
	if got.SecondsPerSDPixelStep != want.sdRate {
		t.Fatalf("expected sd rate %g, got %g", want.sdRate, got.SecondsPerSDPixelStep)
	}
	if got.OverheadSeconds != want.sdOverhead {
		t.Fatalf("expected overhead %g, got %g", want.sdOverhead, got.OverheadSeconds)
	}
}

func TestGetSDExecutionTimeCoefficientsGPUMatchesPricing(t *testing.T) {
	initTaskPricingTestStore(t)
	model := "stabilityai/stable-diffusion-xl-base-1.0"
	gpuName := "NVIDIA GeForce RTX 4090"
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: gpuName, GPUVram: 24,
		ModelName: model, ExecutionDType: models.AutoExecutionDType,
		SDOverheadSeconds: 30, SDFormulaVersion: sdFormulaVersion, SecondsPerSDPixelStep: 0.2, SDSuccessSamples: 1,
	})
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: "Other", GPUVram: 24,
		ModelName: model, ExecutionDType: models.AutoExecutionDType,
		SDOverheadSeconds: 30, SDFormulaVersion: sdFormulaVersion, SecondsPerSDPixelStep: 0.8, SDSuccessSamples: 1,
	})

	query := TaskExecutionTimeQuery{ModelName: model, GPUName: gpuName, GPUVRAM: 24}
	got := GetSDExecutionTimeCoefficients(query)
	want := getTaskPricingParameters(&models.InferenceTask{
		TaskType: models.TaskTypeSD, ModelName: model, RequestedDType: models.AutoExecutionDType,
		RequiredGPU: gpuName, RequiredGPUVRAM: 24,
	})
	if got.SecondsPerSDPixelStep != want.sdRate {
		t.Fatalf("expected sd rate %g, got %g", want.sdRate, got.SecondsPerSDPixelStep)
	}
	if got.OverheadSeconds != want.sdOverhead {
		t.Fatalf("expected overhead %g, got %g", want.sdOverhead, got.OverheadSeconds)
	}
}

func TestGetExecutionTimeCoefficientsUnknownModelUsesInitial(t *testing.T) {
	initTaskPricingTestStore(t)
	initial := initialExecutionParameters()

	sd := GetSDExecutionTimeCoefficients(TaskExecutionTimeQuery{ModelName: "unknown/sd", MinVRAM: 24})
	if sd.OverheadSeconds != initial.sdOverhead || sd.SecondsPerSDPixelStep != initial.sdRate {
		t.Fatalf("unexpected unknown SD coefficients: %+v", sd)
	}

	llm := GetLLMExecutionTimeCoefficients(TaskExecutionTimeQuery{ModelName: "unknown/llm", MinVRAM: 24})
	if llm.ConstantSeconds != initial.llm.constantSeconds ||
		llm.SecondsPerInputToken != initial.llm.secondsPerInputByte*llmInputBytesPerPublicToken ||
		llm.SecondsPerOutputToken != initial.llm.secondsPerOutputToken ||
		llm.ModelSwitchSeconds != initial.llm.modelSwitchSeconds ||
		llm.SecondsPerImage != initial.llm.secondsPerImage ||
		llm.SecondsPerMegapixel != initial.llm.secondsPerMegapixel {
		t.Fatalf("unexpected unknown LLM coefficients: %+v", llm)
	}
}

func TestGetLLMExecutionTimeCoefficientsConvertsInputByteToToken(t *testing.T) {
	initTaskPricingTestStore(t)
	model := "qwen/qwen3.6-7b"
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeLLM, GPUName: "A", GPUVram: 24,
		ModelName: model, ExecutionDType: models.AutoExecutionDType,
		LLMConstantSeconds: 11, LLMSecondsPerInputByte: 0.0002, LLMSecondsPerOutputToken: 0.3,
		LLMModelSwitchSeconds: 40, LLMSecondsPerImage: 7, LLMSecondsPerMegapixel: 9,
		LLMSuccessSamples: 1,
	})

	got := GetLLMExecutionTimeCoefficients(TaskExecutionTimeQuery{ModelName: model, MinVRAM: 24})
	want := getTaskPricingParameters(&models.InferenceTask{
		TaskType: models.TaskTypeLLM, ModelName: model, RequestedDType: models.AutoExecutionDType, MinVRAM: 24,
	})
	if got.ConstantSeconds != want.llm.constantSeconds ||
		got.SecondsPerOutputToken != want.llm.secondsPerOutputToken ||
		got.ModelSwitchSeconds != want.llm.modelSwitchSeconds ||
		got.SecondsPerImage != want.llm.secondsPerImage ||
		got.SecondsPerMegapixel != want.llm.secondsPerMegapixel {
		t.Fatalf("LLM coefficients diverged from pricing parameters: got %+v want %+v", got, want.llm)
	}
	if got.SecondsPerInputToken != want.llm.secondsPerInputByte*4 {
		t.Fatalf("expected seconds_per_input_token %g, got %g", want.llm.secondsPerInputByte*4, got.SecondsPerInputToken)
	}
}

func TestGetSDExecutionTimeCoefficientsExplicitDTypeMatchesOnlyThatRecord(t *testing.T) {
	initTaskPricingTestStore(t)
	model := "stabilityai/stable-diffusion-xl-base-1.0"
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: "A", GPUVram: 24,
		ModelName: model, ExecutionDType: "float16",
		SDOverheadSeconds: 30, SDFormulaVersion: sdFormulaVersion, SecondsPerSDPixelStep: 0.1, SDSuccessSamples: 1,
	})
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeSD, GPUName: "A", GPUVram: 24,
		ModelName: model, ExecutionDType: "bfloat16",
		SDOverheadSeconds: 30, SDFormulaVersion: sdFormulaVersion, SecondsPerSDPixelStep: 0.5, SDSuccessSamples: 1,
	})

	explicit := GetSDExecutionTimeCoefficients(TaskExecutionTimeQuery{
		ModelName: model, RequestedDType: "BFLOAT16", MinVRAM: 24,
	})
	if math.Abs(explicit.SecondsPerSDPixelStep-0.5) > 1e-12 {
		t.Fatalf("expected explicit dtype rate 0.5, got %g", explicit.SecondsPerSDPixelStep)
	}

	auto := GetSDExecutionTimeCoefficients(TaskExecutionTimeQuery{ModelName: model, MinVRAM: 24})
	if math.Abs(auto.SecondsPerSDPixelStep-0.3) > 1e-12 {
		t.Fatalf("expected auto dtype average 0.3, got %g", auto.SecondsPerSDPixelStep)
	}
}

func TestGetLLMExecutionTimeCoefficientsGPUMatchesPricing(t *testing.T) {
	initTaskPricingTestStore(t)
	model := "qwen/qwen3.6-7b"
	gpuName := "NVIDIA GeForce RTX 4090"
	putTestCalibration(models.GPUExecutionCalibration{
		TaskType: models.TaskTypeLLM, GPUName: gpuName, GPUVram: 24,
		ModelName: model, ExecutionDType: models.AutoExecutionDType, QuantizeBits: 4,
		LLMConstantSeconds: 12, LLMSecondsPerInputByte: 0.0003, LLMSecondsPerOutputToken: 0.2,
		LLMModelSwitchSeconds: 50, LLMSecondsPerImage: 8, LLMSecondsPerMegapixel: 6,
		LLMSuccessSamples: 1,
	})

	query := TaskExecutionTimeQuery{
		ModelName: model, GPUName: gpuName, GPUVRAM: 24, RequestedDType: "auto", QuantizeBits: 4,
	}
	got := GetLLMExecutionTimeCoefficients(query)
	want := getTaskPricingParameters(&models.InferenceTask{
		TaskType: models.TaskTypeLLM, ModelName: model, RequestedDType: models.AutoExecutionDType,
		QuantizeBits: 4, RequiredGPU: gpuName, RequiredGPUVRAM: 24,
	})
	if got.SecondsPerInputToken != want.llm.secondsPerInputByte*llmInputBytesPerPublicToken {
		t.Fatalf("expected token coefficient %g, got %g", want.llm.secondsPerInputByte*llmInputBytesPerPublicToken, got.SecondsPerInputToken)
	}
	if got.ConstantSeconds != want.llm.constantSeconds {
		t.Fatalf("expected constant %g, got %g", want.llm.constantSeconds, got.ConstantSeconds)
	}
}
