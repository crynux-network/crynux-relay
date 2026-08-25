package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestNormalizePrivateKey(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
		want       string
	}{
		{
			name:       "without prefix",
			privateKey: "abcdef",
			want:       "abcdef",
		},
		{
			name:       "with lowercase prefix",
			privateKey: "0xabcdef",
			want:       "abcdef",
		},
		{
			name:       "with uppercase prefix",
			privateKey: "  0Xabcdef  ",
			want:       "abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePrivateKey(tt.privateKey); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestInitConfigNormalizesPrivateKeyFromFile(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := t.TempDir()
	privateKey := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKeyFile := filepath.Join(dir, "private_key")
	jwtKeyFile := filepath.Join(dir, "jwt_key")
	macKeyFile := filepath.Join(dir, "mac_key")

	writeTestFile(t, privateKeyFile, "0x"+privateKey)
	writeTestFile(t, jwtKeyFile, "jwt-secret")
	writeTestFile(t, macKeyFile, "mac-secret")

	content := fmt.Sprintf(`environment: debug
blockchains:
  testnet:
    rps: 1
    rpc_endpoint: "http://localhost:8545"
    account:
      address: %q
      private_key_file: %q
    contracts:
      benefit_address: "0x0000000000000000000000000000000000000001"
      node_staking: "0x0000000000000000000000000000000000000002"
    max_withdrawals_per_day: 10
http:
  max_body_bytes: 33554432
  jwt:
    secret_key_file: %q
mac:
  secret_key_file: %q
stats:
  init_start_time: "2026-01-01T00:00:00Z"
network_flops:
  gpu_flops_file: "config/gpu_flops.json"
task:
  passive_slash_mode: true
  history_cleanup_batch_size: 2000
task_pricing:
  initial_sd_overhead_seconds: 30
  initial_seconds_per_sd_pixel_step: 0.00003814697265625
  initial_llm_constant_seconds: 30
  initial_llm_seconds_per_input_byte: 0.0001
  initial_llm_seconds_per_output_token: 0.1
  initial_llm_model_switch_seconds: 120
  initial_llm_seconds_per_image: 10
  initial_llm_seconds_per_megapixel: 5
  calibration_alpha: 0.1
  calibration_regularization: 0.000000001
  calibration_max_positive_residual_multiple: 3
  calibration_warmup_success_samples: 10
  calibration_flush_interval_seconds: 3600
  default_llm_max_new_tokens: 256
  base_vram: 8
  queue_timeout_seconds: 21600
  app_validation_timeout_seconds: 600
  result_upload_timeout_seconds: 600
  timeout_multiplier: 2
  min_execution_timeout_seconds: 60
  max_execution_timeout_seconds: 7200
  queued_task_priority_snapshot_interval_seconds: 300
task_matching:
  batch_size: 100
  tick_interval_seconds: 2
model_distribution:
  controller_interval_seconds: 60
  demand_window_seconds: 1800
  safety_factor: 2.0
  min_nodes: 1
  max_nodes: 10
  download_timeout_seconds: 1800
qos:
  tracing_max_task_events: 50
  penalty_factor: 0.3
  first_timeout_penalty_factor: 0.95
  first_timeout_health_threshold: 0.99
  success_boost: 0.15
  recovery_tau_minutes: 30
  health_exclude_enter_threshold: 0.2
  health_exclude_exit_threshold: 0.8
staking_score:
  locked_emission_coefficient: 1.0
`, addressFromPrivateKey(t, privateKey), filepath.ToSlash(privateKeyFile), filepath.ToSlash(jwtKeyFile), filepath.ToSlash(macKeyFile))
	writeTestFile(t, filepath.Join(dir, "config.yml"), content)

	if err := InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	blockchain, ok := GetConfig().Blockchains["testnet"]
	if !ok {
		t.Fatal("expected testnet blockchain config")
	}
	if blockchain.Account.PrivateKey != privateKey {
		t.Fatalf("expected normalized private key %s, got %s", privateKey, blockchain.Account.PrivateKey)
	}
}

func TestInitConfigHonorsPassiveSlashModeFalse(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := writeConfigTestFiles(t, false, true)
	if err := InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	if GetConfig().Task.PassiveSlashMode == nil {
		t.Fatal("expected passive slash mode to be configured")
	}
	if *GetConfig().Task.PassiveSlashMode {
		t.Fatal("expected passive slash mode false to be honored")
	}
}

func TestInitConfigRequiresPassiveSlashMode(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := writeConfigTestFiles(t, false, false)
	if err := InitConfig(dir); err == nil {
		t.Fatal("expected missing task.passive_slash_mode to fail config initialization")
	}
}

func TestInitConfigRequiresInitialSDOverheadSeconds(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := writeConfigTestFiles(t, true, true)
	configPath := filepath.Join(dir, "config.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	updated := strings.Replace(string(content), "  initial_sd_overhead_seconds: 30\n", "", 1)
	if updated == string(content) {
		t.Fatal("expected to remove initial_sd_overhead_seconds from test config")
	}
	writeTestFile(t, configPath, updated)
	if err := InitConfig(dir); err == nil {
		t.Fatal("expected missing task_pricing.initial_sd_overhead_seconds to fail config initialization")
	}
}

func TestInitConfigRequiresQueuedTaskPrioritySnapshotIntervalSeconds(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := writeConfigTestFiles(t, true, true)
	configPath := filepath.Join(dir, "config.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	updated := strings.Replace(string(content), "  queued_task_priority_snapshot_interval_seconds: 300\n", "", 1)
	if updated == string(content) {
		t.Fatal("expected to remove queued_task_priority_snapshot_interval_seconds from test config")
	}
	writeTestFile(t, configPath, updated)
	if err := InitConfig(dir); err == nil {
		t.Fatal("expected missing queued_task_priority_snapshot_interval_seconds to fail config initialization")
	}
}

func TestInitConfigRejectsZeroQueuedTaskPrioritySnapshotIntervalSeconds(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := writeConfigTestFiles(t, true, true)
	configPath := filepath.Join(dir, "config.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	updated := strings.Replace(
		string(content),
		"  queued_task_priority_snapshot_interval_seconds: 300\n",
		"  queued_task_priority_snapshot_interval_seconds: 0\n",
		1,
	)
	if updated == string(content) {
		t.Fatal("expected to replace queued_task_priority_snapshot_interval_seconds in test config")
	}
	writeTestFile(t, configPath, updated)
	if err := InitConfig(dir); err == nil {
		t.Fatal("expected zero queued_task_priority_snapshot_interval_seconds to fail config initialization")
	}
}

func TestInitConfigLoadsQueuedTaskPrioritySnapshotIntervalSeconds(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := writeConfigTestFiles(t, true, true)
	if err := InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	if GetConfig().TaskPricing.QueuedTaskPrioritySnapshotIntervalSeconds != 300 {
		t.Fatalf(
			"expected queued_task_priority_snapshot_interval_seconds 300, got %d",
			GetConfig().TaskPricing.QueuedTaskPrioritySnapshotIntervalSeconds,
		)
	}
}

func TestInitConfigRequiresQosTracingMaxTaskEvents(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := t.TempDir()
	privateKey := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKeyFile := filepath.Join(dir, "private_key")
	jwtKeyFile := filepath.Join(dir, "jwt_key")
	macKeyFile := filepath.Join(dir, "mac_key")

	writeTestFile(t, privateKeyFile, "0x"+privateKey)
	writeTestFile(t, jwtKeyFile, "jwt-secret")
	writeTestFile(t, macKeyFile, "mac-secret")

	content := fmt.Sprintf(`environment: debug
blockchains:
  testnet:
    rps: 1
    rpc_endpoint: "http://localhost:8545"
    account:
      address: %q
      private_key_file: %q
    contracts:
      benefit_address: "0x0000000000000000000000000000000000000001"
      node_staking: "0x0000000000000000000000000000000000000002"
    max_withdrawals_per_day: 10
http:
  max_body_bytes: 33554432
  jwt:
    secret_key_file: %q
mac:
  secret_key_file: %q
stats:
  init_start_time: "2026-01-01T00:00:00Z"
network_flops:
  gpu_flops_file: "config/gpu_flops.json"
task:
  passive_slash_mode: true
  history_cleanup_batch_size: 2000
task_pricing:
  initial_sd_overhead_seconds: 30
  initial_seconds_per_sd_pixel_step: 0.00003814697265625
  initial_llm_constant_seconds: 30
  initial_llm_seconds_per_input_byte: 0.0001
  initial_llm_seconds_per_output_token: 0.1
  initial_llm_model_switch_seconds: 120
  initial_llm_seconds_per_image: 10
  initial_llm_seconds_per_megapixel: 5
  calibration_alpha: 0.1
  calibration_regularization: 0.000000001
  calibration_max_positive_residual_multiple: 3
  calibration_warmup_success_samples: 10
  calibration_flush_interval_seconds: 3600
  default_llm_max_new_tokens: 256
  base_vram: 8
  queue_timeout_seconds: 21600
  app_validation_timeout_seconds: 600
  result_upload_timeout_seconds: 600
  timeout_multiplier: 2
  min_execution_timeout_seconds: 60
  max_execution_timeout_seconds: 7200
  queued_task_priority_snapshot_interval_seconds: 300
task_matching:
  batch_size: 100
  tick_interval_seconds: 2
model_distribution:
  controller_interval_seconds: 60
  demand_window_seconds: 1800
  safety_factor: 2.0
  min_nodes: 1
  max_nodes: 10
  download_timeout_seconds: 1800
staking_score:
  locked_emission_coefficient: 1.0
`, addressFromPrivateKey(t, privateKey), filepath.ToSlash(privateKeyFile), filepath.ToSlash(jwtKeyFile), filepath.ToSlash(macKeyFile))
	writeTestFile(t, filepath.Join(dir, "config.yml"), content)

	if err := InitConfig(dir); err == nil {
		t.Fatal("expected missing qos.tracing_max_task_events to fail config initialization")
	}
}

func TestCheckQosConfigHealthParameters(t *testing.T) {
	original := appConfig
	t.Cleanup(func() {
		appConfig = original
	})

	validConfig := func() *AppConfig {
		cfg := &AppConfig{}
		cfg.QoS.TracingMaxTaskEvents = 50
		cfg.QoS.PenaltyFactor = 0.3
		cfg.QoS.FirstTimeoutPenaltyFactor = 0.95
		cfg.QoS.FirstTimeoutHealthThreshold = 0.99
		cfg.QoS.SuccessBoost = 0.15
		cfg.QoS.RecoveryTauMinutes = 30
		cfg.QoS.HealthExcludeEnterThreshold = 0.2
		cfg.QoS.HealthExcludeExitThreshold = 0.8
		return cfg
	}

	tests := []struct {
		name   string
		mutate func(*AppConfig)
	}{
		{name: "valid", mutate: func(*AppConfig) {}},
		{name: "enter zero", mutate: func(cfg *AppConfig) { cfg.QoS.HealthExcludeEnterThreshold = 0 }},
		{name: "enter equals exit", mutate: func(cfg *AppConfig) { cfg.QoS.HealthExcludeEnterThreshold = cfg.QoS.HealthExcludeExitThreshold }},
		{name: "exit above one", mutate: func(cfg *AppConfig) { cfg.QoS.HealthExcludeExitThreshold = 1.1 }},
		{name: "penalty zero", mutate: func(cfg *AppConfig) { cfg.QoS.PenaltyFactor = 0 }},
		{name: "first penalty above one", mutate: func(cfg *AppConfig) { cfg.QoS.FirstTimeoutPenaltyFactor = 1.1 }},
		{name: "first threshold zero", mutate: func(cfg *AppConfig) { cfg.QoS.FirstTimeoutHealthThreshold = 0 }},
		{name: "success boost negative", mutate: func(cfg *AppConfig) { cfg.QoS.SuccessBoost = -0.1 }},
		{name: "success boost above one", mutate: func(cfg *AppConfig) { cfg.QoS.SuccessBoost = 1.1 }},
		{name: "recovery tau zero", mutate: func(cfg *AppConfig) { cfg.QoS.RecoveryTauMinutes = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appConfig = validConfig()
			tt.mutate(appConfig)
			err := checkQosConfig()
			if tt.name == "valid" && err != nil {
				t.Fatalf("expected valid config, got %v", err)
			}
			if tt.name != "valid" && err == nil {
				t.Fatal("expected invalid config")
			}
		})
	}
}

func TestInitConfigRequiresNetworkFLOPSFile(t *testing.T) {
	t.Cleanup(func() {
		appConfig = nil
	})

	dir := t.TempDir()
	privateKey := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKeyFile := filepath.Join(dir, "private_key")
	jwtKeyFile := filepath.Join(dir, "jwt_key")
	macKeyFile := filepath.Join(dir, "mac_key")

	writeTestFile(t, privateKeyFile, "0x"+privateKey)
	writeTestFile(t, jwtKeyFile, "jwt-secret")
	writeTestFile(t, macKeyFile, "mac-secret")

	content := fmt.Sprintf(`environment: debug
blockchains:
  testnet:
    rps: 1
    rpc_endpoint: "http://localhost:8545"
    account:
      address: %q
      private_key_file: %q
    contracts:
      benefit_address: "0x0000000000000000000000000000000000000001"
      node_staking: "0x0000000000000000000000000000000000000002"
    max_withdrawals_per_day: 10
http:
  max_body_bytes: 33554432
  jwt:
    secret_key_file: %q
mac:
  secret_key_file: %q
stats:
  init_start_time: "2026-01-01T00:00:00Z"
task:
  passive_slash_mode: true
  history_cleanup_batch_size: 2000
task_pricing:
  initial_sd_overhead_seconds: 30
  initial_seconds_per_sd_pixel_step: 0.00003814697265625
  initial_llm_constant_seconds: 30
  initial_llm_seconds_per_input_byte: 0.0001
  initial_llm_seconds_per_output_token: 0.1
  initial_llm_model_switch_seconds: 120
  initial_llm_seconds_per_image: 10
  initial_llm_seconds_per_megapixel: 5
  calibration_alpha: 0.1
  calibration_regularization: 0.000000001
  calibration_max_positive_residual_multiple: 3
  calibration_warmup_success_samples: 10
  calibration_flush_interval_seconds: 3600
  default_llm_max_new_tokens: 256
  base_vram: 8
  queue_timeout_seconds: 21600
  app_validation_timeout_seconds: 600
  result_upload_timeout_seconds: 600
  timeout_multiplier: 2
  min_execution_timeout_seconds: 60
  max_execution_timeout_seconds: 7200
  queued_task_priority_snapshot_interval_seconds: 300
task_matching:
  batch_size: 100
  tick_interval_seconds: 2
model_distribution:
  controller_interval_seconds: 60
  demand_window_seconds: 1800
  safety_factor: 2.0
  min_nodes: 1
  max_nodes: 10
  download_timeout_seconds: 1800
qos:
  tracing_max_task_events: 50
  penalty_factor: 0.3
  first_timeout_penalty_factor: 0.95
  first_timeout_health_threshold: 0.99
  success_boost: 0.15
  recovery_tau_minutes: 30
  health_exclude_enter_threshold: 0.2
  health_exclude_exit_threshold: 0.8
staking_score:
  locked_emission_coefficient: 1.0
`, addressFromPrivateKey(t, privateKey), filepath.ToSlash(privateKeyFile), filepath.ToSlash(jwtKeyFile), filepath.ToSlash(macKeyFile))
	writeTestFile(t, filepath.Join(dir, "config.yml"), content)

	if err := InitConfig(dir); err == nil {
		t.Fatal("expected missing network_flops.gpu_flops_file to fail config initialization")
	}
}

func writeConfigTestFiles(t *testing.T, passiveSlashMode bool, includePassiveSlashMode bool) string {
	t.Helper()
	dir := t.TempDir()
	privateKey := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKeyFile := filepath.Join(dir, "private_key")
	jwtKeyFile := filepath.Join(dir, "jwt_key")
	macKeyFile := filepath.Join(dir, "mac_key")

	writeTestFile(t, privateKeyFile, "0x"+privateKey)
	writeTestFile(t, jwtKeyFile, "jwt-secret")
	writeTestFile(t, macKeyFile, "mac-secret")

	taskConfig := "task:\n  history_cleanup_batch_size: 2000\n"
	if includePassiveSlashMode {
		taskConfig = fmt.Sprintf("task:\n  passive_slash_mode: %t\n  history_cleanup_batch_size: 2000\n", passiveSlashMode)
	}
	content := fmt.Sprintf(`environment: debug
blockchains:
  testnet:
    rps: 1
    rpc_endpoint: "http://localhost:8545"
    account:
      address: %q
      private_key_file: %q
    contracts:
      benefit_address: "0x0000000000000000000000000000000000000001"
      node_staking: "0x0000000000000000000000000000000000000002"
    max_withdrawals_per_day: 10
http:
  max_body_bytes: 33554432
  jwt:
    secret_key_file: %q
mac:
  secret_key_file: %q
stats:
  init_start_time: "2026-01-01T00:00:00Z"
network_flops:
  gpu_flops_file: "config/gpu_flops.json"
task_pricing:
  initial_sd_overhead_seconds: 30
  initial_seconds_per_sd_pixel_step: 0.00003814697265625
  initial_llm_constant_seconds: 30
  initial_llm_seconds_per_input_byte: 0.0001
  initial_llm_seconds_per_output_token: 0.1
  initial_llm_model_switch_seconds: 120
  initial_llm_seconds_per_image: 10
  initial_llm_seconds_per_megapixel: 5
  calibration_alpha: 0.1
  calibration_regularization: 0.000000001
  calibration_max_positive_residual_multiple: 3
  calibration_warmup_success_samples: 10
  calibration_flush_interval_seconds: 3600
  default_llm_max_new_tokens: 256
  base_vram: 8
  queue_timeout_seconds: 21600
  app_validation_timeout_seconds: 600
  result_upload_timeout_seconds: 600
  timeout_multiplier: 2
  min_execution_timeout_seconds: 60
  max_execution_timeout_seconds: 7200
  queued_task_priority_snapshot_interval_seconds: 300
task_matching:
  batch_size: 100
  tick_interval_seconds: 2
model_distribution:
  controller_interval_seconds: 60
  demand_window_seconds: 1800
  safety_factor: 2.0
  min_nodes: 1
  max_nodes: 10
  download_timeout_seconds: 1800
qos:
  tracing_max_task_events: 50
  penalty_factor: 0.3
  first_timeout_penalty_factor: 0.95
  first_timeout_health_threshold: 0.99
  success_boost: 0.15
  recovery_tau_minutes: 30
  health_exclude_enter_threshold: 0.2
  health_exclude_exit_threshold: 0.8
staking_score:
  locked_emission_coefficient: 1.0
%s`, addressFromPrivateKey(t, privateKey), filepath.ToSlash(privateKeyFile), filepath.ToSlash(jwtKeyFile), filepath.ToSlash(macKeyFile), taskConfig)
	writeTestFile(t, filepath.Join(dir, "config.yml"), content)
	return dir
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func addressFromPrivateKey(t *testing.T, privateKeyHex string) string {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	return crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
}
