package service

import (
	"crynux_relay/config"
	"os"
	"path/filepath"
	"testing"
)

// initServiceTestConfig writes a minimal valid config with an in-memory
// sqlite database and initializes config and database for service tests.
func initServiceTestConfig(t *testing.T) {
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
		"  batch_create_max_items: 100\n" +
		"  batch_status_max_items: 500\n" +
		"  batch_validate_max_items: 100\n" +
		"  batch_abort_max_items: 100\n" +
		"  result_max_uncompressed_bytes: 1073741824\n" +
		"  timeout_query_batch_size: 100\n" +
		taskPricingMatchingTestConfigYAML +
		"qos:\n" +
		"  tracing_max_task_events: 50\n" +
		qosHealthTestConfigYAML
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	if err := config.InitDB(config.GetConfig()); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
}

// taskPricingMatchingTestConfigYAML holds the required task_pricing,
// task_matching, model_distribution and staking_score config sections shared
// by inline test configurations.
const taskPricingMatchingTestConfigYAML = "staking_score:\n" +
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
	"  download_timeout_seconds: 1800\n"

const qosHealthTestConfigYAML = "  penalty_factor: 0.3\n" +
	"  first_timeout_penalty_factor: 0.95\n" +
	"  first_timeout_health_threshold: 0.99\n" +
	"  success_boost: 0.15\n" +
	"  recovery_tau_minutes: 30\n" +
	"  health_exclude_enter_threshold: 0.2\n" +
	"  health_exclude_exit_threshold: 0.8\n"
