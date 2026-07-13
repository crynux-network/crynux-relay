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
		"task:\n" +
		"  passive_slash_mode: true\n" +
		taskPricingMatchingTestConfigYAML +
		"qos:\n" +
		"  tracing_max_task_events: 50\n"
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

// taskPricingMatchingTestConfigYAML holds the required task_pricing and
// task_matching config sections shared by inline test configurations.
const taskPricingMatchingTestConfigYAML = "task_pricing:\n" +
	"  overhead_seconds: 30\n" +
	"  initial_seconds_per_sd_unit: 10\n" +
	"  initial_seconds_per_llm_token: 0.1\n" +
	"  calibration_alpha: 0.1\n" +
	"  default_llm_max_new_tokens: 256\n" +
	"  base_vram: 8\n" +
	"task_matching:\n" +
	"  batch_size: 100\n" +
	"  tick_interval_seconds: 2\n"
