package models

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	dbmodels "crynux_relay/models"
	"crynux_relay/service"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func uint64Ptr(v uint64) *uint64 { return &v }

func strPtr(v string) *string { return &v }

func testExecutionTimeContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v2/models/sd/execution-time", nil)
	return c
}

func requireValidationError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	validation, ok := err.(*response.ValidationErrorResponse)
	if !ok {
		t.Fatalf("expected ValidationErrorResponse, got %T: %v", err, err)
	}
	if validation.GetFieldName() != field {
		t.Fatalf("expected field %q, got %q", field, validation.GetFieldName())
	}
}

func initExecutionTimeAPITest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	content := "environment: test\n" +
		"db:\n" +
		"  driver: sqlite\n" +
		"  connection: ':memory:'\n" +
		"  log:\n" +
		"    level: info\n" +
		"    output: stdout\n" +
		"blockchains: {}\n" +
		"http:\n" +
		"  max_body_bytes: 33554432\n" +
		"stats:\n" +
		"  init_start_time: \"2026-01-01T00:00:00Z\"\n" +
		"network_flops:\n" +
		"  gpu_flops_file: \"config/gpu_flops.json\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"  history_cleanup_batch_size: 2000\n" +
		"staking_score:\n" +
		"  locked_emission_coefficient: 1.0\n" +
		"task_pricing:\n" +
		"  initial_sd_overhead_seconds: 30\n" +
		"  initial_seconds_per_sd_pixel_step: 0.00003814697265625\n" +
		"  initial_llm_constant_seconds: 30\n" +
		"  initial_llm_seconds_per_input_byte: 0.0001\n" +
		"  initial_llm_seconds_per_output_token: 0.1\n" +
		"  initial_llm_model_switch_seconds: 120\n" +
		"  initial_llm_seconds_per_image: 10\n" +
		"  initial_llm_seconds_per_megapixel: 5\n" +
		"  calibration_alpha: 0.1\n" +
		"  calibration_regularization: 0.000000001\n" +
		"  calibration_max_positive_residual_multiple: 3\n" +
		"  calibration_warmup_success_samples: 10\n" +
		"  calibration_flush_interval_seconds: 3600\n" +
		"  default_llm_max_new_tokens: 256\n" +
		"  base_vram: 8\n" +
		"  queue_timeout_seconds: 21600\n" +
		"  app_validation_timeout_seconds: 600\n" +
		"  result_upload_timeout_seconds: 600\n" +
		"  timeout_multiplier: 2\n" +
		"  min_execution_timeout_seconds: 60\n" +
		"  max_execution_timeout_seconds: 7200\n" +
		"  queued_task_priority_snapshot_interval_seconds: 300\n" +
		"task_matching:\n" +
		"  batch_size: 100\n" +
		"  tick_interval_seconds: 2\n" +
		"model_distribution:\n" +
		"  controller_interval_seconds: 60\n" +
		"  demand_window_seconds: 1800\n" +
		"  safety_factor: 2.0\n" +
		"  min_nodes: 1\n" +
		"  max_nodes: 10\n" +
		"  download_timeout_seconds: 1800\n" +
		"qos:\n" +
		"  tracing_max_task_events: 50\n" +
		"  penalty_factor: 0.3\n" +
		"  first_timeout_penalty_factor: 0.95\n" +
		"  first_timeout_health_threshold: 0.99\n" +
		"  success_boost: 0.15\n" +
		"  recovery_tau_minutes: 30\n" +
		"  health_exclude_enter_threshold: 0.2\n" +
		"  health_exclude_exit_threshold: 0.8\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	if err := config.InitDB(config.GetConfig()); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	if err := config.GetDB().AutoMigrate(&dbmodels.GPUExecutionCalibration{}); err != nil {
		t.Fatalf("failed to migrate calibrations: %v", err)
	}
	if err := service.InitTaskPricing(context.Background(), config.GetDB()); err != nil {
		t.Fatalf("failed to init task pricing: %v", err)
	}
}

func TestGetSDExecutionTimeMissingModel(t *testing.T) {
	_, err := GetSDExecutionTime(testExecutionTimeContext(), &GetSDExecutionTimeInput{MinVRAM: uint64Ptr(24)})
	requireValidationError(t, err, "model")
}

func TestGetSDExecutionTimeBothSelectionModes(t *testing.T) {
	_, err := GetSDExecutionTime(testExecutionTimeContext(), &GetSDExecutionTimeInput{
		Model:   "stabilityai/stable-diffusion-xl-base-1.0",
		MinVRAM: uint64Ptr(24),
		GPUName: strPtr("NVIDIA GeForce RTX 4090"),
		GPUVRAM: uint64Ptr(24),
	})
	requireValidationError(t, err, "min_vram")
}

func TestGetSDExecutionTimeOnlyGPUName(t *testing.T) {
	_, err := GetSDExecutionTime(testExecutionTimeContext(), &GetSDExecutionTimeInput{
		Model:   "stabilityai/stable-diffusion-xl-base-1.0",
		GPUName: strPtr("NVIDIA GeForce RTX 4090"),
	})
	requireValidationError(t, err, "gpu_vram")
}

func TestGetSDExecutionTimeMinVRAMZero(t *testing.T) {
	_, err := GetSDExecutionTime(testExecutionTimeContext(), &GetSDExecutionTimeInput{
		Model:   "stabilityai/stable-diffusion-xl-base-1.0",
		MinVRAM: uint64Ptr(0),
	})
	requireValidationError(t, err, "min_vram")
}

func TestGetSDExecutionTimeSuccess(t *testing.T) {
	initExecutionTimeAPITest(t)
	resp, err := GetSDExecutionTime(testExecutionTimeContext(), &GetSDExecutionTimeInput{
		Model:   "stabilityai/stable-diffusion-xl-base-1.0",
		MinVRAM: uint64Ptr(24),
	})
	if err != nil {
		t.Fatalf("GetSDExecutionTime failed: %v", err)
	}
	cfg := config.GetConfig().TaskPricing
	if resp.Data.OverheadSeconds != cfg.InitialSDOverheadSeconds {
		t.Fatalf("unexpected overhead_seconds: %g", resp.Data.OverheadSeconds)
	}
	if resp.Data.SecondsPerSDPixelStep != cfg.InitialSecondsPerSDPixelStep {
		t.Fatalf("unexpected seconds_per_sd_pixel_step: %g", resp.Data.SecondsPerSDPixelStep)
	}
}

func TestGetSDExecutionTimeReturnsFittedOverhead(t *testing.T) {
	initExecutionTimeAPITest(t)
	model := "stabilityai/stable-diffusion-xl-base-1.0"
	record := dbmodels.GPUExecutionCalibration{
		TaskType:              dbmodels.TaskTypeSD,
		GPUName:               "A100",
		GPUVram:               40,
		ModelName:             model,
		ExecutionDType:        "auto",
		MinVRAMRequirement:    8,
		MaxVRAMRequirement:    40,
		SDOverheadSeconds:     42,
		SDFormulaVersion:      1,
		SecondsPerSDPixelStep: 0.0001,
		SDSuccessSamples:      10,
		LLMFormulaVersion:     2,
	}
	if err := config.GetDB().Create(&record).Error; err != nil {
		t.Fatalf("seed fitted calibration: %v", err)
	}
	if err := service.InitTaskPricing(context.Background(), config.GetDB()); err != nil {
		t.Fatalf("reload task pricing: %v", err)
	}
	resp, err := GetSDExecutionTime(testExecutionTimeContext(), &GetSDExecutionTimeInput{
		Model:   model,
		MinVRAM: uint64Ptr(24),
	})
	if err != nil {
		t.Fatalf("GetSDExecutionTime failed: %v", err)
	}
	if resp.Data.OverheadSeconds != 42 {
		t.Fatalf("expected fitted overhead_seconds 42, got %g", resp.Data.OverheadSeconds)
	}
	if resp.Data.SecondsPerSDPixelStep != 0.0001 {
		t.Fatalf("expected fitted seconds_per_sd_pixel_step 0.0001, got %g", resp.Data.SecondsPerSDPixelStep)
	}
}

func TestGetLLMExecutionTimeSuccess(t *testing.T) {
	initExecutionTimeAPITest(t)
	resp, err := GetLLMExecutionTime(testExecutionTimeContext(), &GetLLMExecutionTimeInput{
		Model:   "qwen/qwen3.6-7b",
		MinVRAM: uint64Ptr(24),
	})
	if err != nil {
		t.Fatalf("GetLLMExecutionTime failed: %v", err)
	}
	cfg := config.GetConfig().TaskPricing
	if resp.Data.ConstantSeconds != cfg.InitialLLMConstantSeconds ||
		resp.Data.SecondsPerInputToken != cfg.InitialLLMSecondsPerInputByte*4 ||
		resp.Data.SecondsPerOutputToken != cfg.InitialLLMSecondsPerOutputToken ||
		resp.Data.ModelSwitchSeconds != cfg.InitialLLMModelSwitchSeconds ||
		resp.Data.SecondsPerImage != cfg.InitialLLMSecondsPerImage ||
		resp.Data.SecondsPerMegapixel != cfg.InitialLLMSecondsPerMegapixel {
		t.Fatalf("unexpected LLM coefficients: %+v", resp.Data)
	}
}
