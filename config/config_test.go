package config

import (
	"fmt"
	"os"
	"path/filepath"
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
      credits: "0x0000000000000000000000000000000000000003"
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
task_pricing:
  overhead_seconds: 30
  initial_seconds_per_sd_unit: 10
  initial_seconds_per_llm_token: 0.1
  calibration_alpha: 0.1
  default_llm_max_new_tokens: 256
  base_vram: 8
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
      credits: "0x0000000000000000000000000000000000000003"
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
task_pricing:
  overhead_seconds: 30
  initial_seconds_per_sd_unit: 10
  initial_seconds_per_llm_token: 0.1
  calibration_alpha: 0.1
  default_llm_max_new_tokens: 256
  base_vram: 8
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

	taskConfig := ""
	if includePassiveSlashMode {
		taskConfig = fmt.Sprintf("task:\n  passive_slash_mode: %t\n", passiveSlashMode)
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
      credits: "0x0000000000000000000000000000000000000003"
http:
  max_body_bytes: 33554432
  jwt:
    secret_key_file: %q
mac:
  secret_key_file: %q
stats:
  init_start_time: "2026-01-01T00:00:00Z"
task_pricing:
  overhead_seconds: 30
  initial_seconds_per_sd_unit: 10
  initial_seconds_per_llm_token: 0.1
  calibration_alpha: 0.1
  default_llm_max_new_tokens: 256
  base_vram: 8
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
