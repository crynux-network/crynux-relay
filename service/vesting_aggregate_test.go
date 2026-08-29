package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const vestingAggregateTestPrivateKey = "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"

func writeVestingAggregateTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func vestingAggregateTestAddressFromPrivateKey(t *testing.T, privateKey string) string {
	t.Helper()
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex()
}

func initVestingAggregateTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	privateKeyFile := filepath.Join(dir, "private_key")
	jwtKeyFile := filepath.Join(dir, "jwt_key")
	macKeyFile := filepath.Join(dir, "mac_key")

	writeVestingAggregateTestFile(t, privateKeyFile, "0x"+vestingAggregateTestPrivateKey)
	writeVestingAggregateTestFile(t, jwtKeyFile, "jwt-secret")
	writeVestingAggregateTestFile(t, macKeyFile, "mac-secret")

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
admin:
  vesting_signer_address: %q
mac:
  secret_key_file: %q
stats:
  init_start_time: "2026-01-01T00:00:00Z"
network_flops:
  gpu_flops_file: "config/gpu_flops.json"
task:
  passive_slash_mode: true
  history_cleanup_batch_size: 2000
  batch_create_max_items: 100
  batch_status_max_items: 500
  batch_validate_max_items: 100
  batch_abort_max_items: 100
  result_max_uncompressed_bytes: 1073741824
  timeout_query_batch_size: 100
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
`, vestingAggregateTestAddressFromPrivateKey(t, vestingAggregateTestPrivateKey), filepath.ToSlash(privateKeyFile), filepath.ToSlash(jwtKeyFile), vestingAggregateTestAddressFromPrivateKey(t, vestingAggregateTestPrivateKey), filepath.ToSlash(macKeyFile))
	writeVestingAggregateTestFile(t, filepath.Join(dir, "config.yml"), content)

	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
}

func signedDelegationVestingInput(t *testing.T, address string, totalAmount string, start time.Time, details []CreateVestingDelegationDetailInput) CreateVestingRecordInput {
	t.Helper()
	payload := vestingSignPayload{
		Address:      address,
		TotalAmount:  totalAmount,
		StartTime:    start.Unix(),
		DurationDays: 180,
		Type:         models.VestingTypeDelegation,
	}
	signature, err := blockchain.NewSignatureVerifier().SignMessage(buildVestingSignMessage(payload), vestingAggregateTestPrivateKey)
	if err != nil {
		t.Fatalf("failed to sign vesting payload: %v", err)
	}
	return CreateVestingRecordInput{
		Address:           address,
		TotalAmount:       totalAmount,
		StartTime:         payload.StartTime,
		DurationDays:      payload.DurationDays,
		Type:              payload.Type,
		AdminSignature:    signature,
		DelegationDetails: details,
	}
}

func TestCreateVestingRecordsMaintainsNodeDelegationEmissionWeeklyTotals(t *testing.T) {
	initVestingAggregateTestConfig(t)
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Node{},
		&models.VestingRecord{},
		&models.VestingDelegationEmissionDetail{},
		&models.NodeDelegationEmissionWeeklyTotal{},
		&models.RelayAccountEvent{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	start := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)
	nodeAddress := "0x0000000000000000000000000000000000000100"
	firstBatch := []CreateVestingRecordInput{
		signedDelegationVestingInput(t, "0x0000000000000000000000000000000000000001", "100", start, []CreateVestingDelegationDetailInput{
			{
				NodeAddress:    nodeAddress,
				Network:        "base",
				TaskFee:        "10",
				EmissionAmount: "100",
				StartTime:      start.Unix(),
			},
		}),
		signedDelegationVestingInput(t, "0x0000000000000000000000000000000000000002", "200", start, []CreateVestingDelegationDetailInput{
			{
				NodeAddress:    nodeAddress,
				Network:        "base",
				TaskFee:        "20",
				EmissionAmount: "200",
				StartTime:      start.Unix(),
			},
		}),
	}
	if _, err := CreateVestingRecords(ctx, db, firstBatch); err != nil {
		t.Fatalf("first CreateVestingRecords failed: %v", err)
	}

	totals, err := models.ListNodeDelegationEmissionWeeklyTotalsByNodeAndStartTimeRange(ctx, db, nodeAddress, start, start.Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("list totals after first batch failed: %v", err)
	}
	if len(totals) != 1 {
		t.Fatalf("expected one aggregate row after first batch, got %d", len(totals))
	}
	if totals[0].EmissionAmount.Int.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected first batch aggregate 300, got %s", totals[0].EmissionAmount.String())
	}

	secondBatch := []CreateVestingRecordInput{
		signedDelegationVestingInput(t, "0x0000000000000000000000000000000000000003", "150", start, []CreateVestingDelegationDetailInput{
			{
				NodeAddress:    nodeAddress,
				Network:        "base",
				TaskFee:        "15",
				EmissionAmount: "150",
				StartTime:      start.Unix(),
			},
		}),
	}
	if _, err := CreateVestingRecords(ctx, db, secondBatch); err != nil {
		t.Fatalf("second CreateVestingRecords failed: %v", err)
	}

	totals, err = models.ListNodeDelegationEmissionWeeklyTotalsByNodeAndStartTimeRange(ctx, db, nodeAddress, start, start.Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("list totals after second batch failed: %v", err)
	}
	if len(totals) != 1 {
		t.Fatalf("expected one aggregate row after second batch, got %d", len(totals))
	}
	if totals[0].EmissionAmount.Int.Cmp(big.NewInt(450)) != 0 {
		t.Fatalf("expected aggregate total 450 after additive batch, got %s", totals[0].EmissionAmount.String())
	}
}
