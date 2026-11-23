package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"crynux_relay/service/huggingface"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
)

func TestTaskInputTimeoutJSON(t *testing.T) {
	input := TaskInput{}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"timeout"`) {
		t.Fatalf("zero timeout must be omitted from signed input: %s", payload)
	}

	input.Timeout = 120
	payload, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"timeout":120`) {
		t.Fatalf("non-zero timeout must be included in signed input: %s", payload)
	}
}

func TestSubmitScoreExecutionDTypeJSONCompatibility(t *testing.T) {
	input := SubmitScoreInput{TaskIDCommitment: "task", Score: "0x01"}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"execution_dtype"`) {
		t.Fatalf("missing dtype must preserve old signed payload: %s", payload)
	}
	input.ExecutionDType = "bfloat16"
	payload, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"execution_dtype":"bfloat16"`) {
		t.Fatalf("reported dtype must be signed: %s", payload)
	}
}

const createTaskTestPrivateKey = "420fcabfd5dbb55215490693062e6e530840c64de837d071f0d9da21aaac861e"

func initCreateTaskAPITest(t *testing.T, schemaURL, hfURL string) {
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
		"task_schema:\n" +
		"  stable_diffusion_inference: \"" + schemaURL + "\"\n" +
		"  gpt_inference: \"" + schemaURL + "\"\n" +
		"  stable_diffusion_finetune_lora: \"" + schemaURL + "\"\n" +
		"  video_inference: \"" + schemaURL + "\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"  video_distance_mean_threshold: 10\n" +
		"  video_distance_max_threshold: 20\n" +
		"  video_audio_distance_threshold: 16\n" +
		"  video_upload_audio_distance_threshold: 2\n" +
		"  history_cleanup_batch_size: 2000\n" +
		"  batch_create_max_items: 100\n" +
		"  batch_status_max_items: 500\n" +
		"  batch_validate_max_items: 100\n" +
		"  batch_abort_max_items: 100\n" +
		"  result_max_uncompressed_bytes: 1073741824\n" +
		"  timeout_query_batch_size: 100\n" +
		"staking_score:\n" +
		"  locked_emission_coefficient: 1.0\n" +
		"task_pricing:\n" +
		"  initial_sd_overhead_seconds: 30\n" +
		"  initial_seconds_per_sd_pixel_step: 0.00003814697265625\n" +
		"  initial_video_overhead_seconds: 30\n" +
		"  initial_seconds_per_video_unit: 0.000001\n" +
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
		"  default_queue_timeout_seconds: 21600\n" +
		"  max_queue_timeout_seconds: 259200\n" +
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
		"huggingface:\n" +
		"  api_base_url: \"" + hfURL + "\"\n" +
		"  timeout_seconds: 5\n" +
		"  cache_ttl_seconds: 300\n" +
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
	if err := config.GetDB().AutoMigrate(
		&models.InferenceTask{},
		&models.ModelFile{},
		&models.NodeModel{},
		&models.Node{},
		&models.ModelRuntimeConstraint{},
		&models.NetworkTaskNumber{},
		&models.RelayAccountEvent{},
	); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}
}

func signCanonicalPayload(t *testing.T, value interface{}) (int64, string) {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(createTaskTestPrivateKey)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}
	timestamp := time.Now().Unix()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var canonical interface{}
	if err := json.Unmarshal(raw, &canonical); err != nil {
		t.Fatalf("unmarshal canonical payload: %v", err)
	}
	canonicalBytes, err := json.Marshal(canonical)
	if err != nil {
		t.Fatalf("marshal canonical payload: %v", err)
	}
	hash := crypto.Keccak256Hash(append(canonicalBytes, []byte(strconv.FormatInt(timestamp, 10))...))
	sig, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	return timestamp, hexutil.Encode(sig)
}

func TestCreateTaskAndBatchRejectUnsupportedRequiredGPUPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.ClearRuntimeConstraintMemoryForTest()

	schemaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"object"}`))
	}))
	defer schemaSrv.Close()

	commitSHA := "0123456789abcdef0123456789abcdef01234567"
	hfCalls := 0
	hfSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hfCalls++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/models/org/nunchaku-sd", "/api/models/org/nunchaku-sd/revision/main":
			_, _ = w.Write([]byte(`{"sha":"` + commitSHA + `","siblings":[{"rfilename":"model_index.json"},{"rfilename":"transformer/diffusion_pytorch_model.safetensors"}]}`))
		case "/org/nunchaku-sd/resolve/" + commitSHA + "/model_index.json":
			_, _ = w.Write([]byte(`{"transformer":["diffusers","SDTransformer2DModel"]}`))
		case "/org/nunchaku-sd/resolve/" + commitSHA + "/transformer/config.json":
			_, _ = w.Write([]byte(`{"quantization_config":{"quant_method":"nunchaku"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer hfSrv.Close()

	initCreateTaskAPITest(t, schemaSrv.URL+"/schema.json", hfSrv.URL)
	service.SetHuggingFaceClientForTest(huggingface.NewClient(hfSrv.URL, 5*time.Second, time.Minute))
	t.Cleanup(func() { service.SetHuggingFaceClientForTest(nil) })

	minVRAM := uint64(24)
	gpuVRAM := uint64(64)
	darwinGPU := "Apple M3 Max+Darwin"
	windowsGPU := "NVIDIA GeForce RTX 4090+Windows"

	// 1. Video task pinning Darwin RequiredGPU is rejected immediately with validation_error on required_gpu (no Hub call).
	videoInput := TaskInput{
		TaskIDCommitment: "0x1111111111111111111111111111111111111111111111111111111111111111",
		TaskArgs:         `{"version":"3.8.0","base_model":{"name":"Wan-AI/Wan2.2-T2V-A14B-Diffusers"},"prompt":"rain","task_config":{"num_frames":81,"steps":4}}`,
		TaskType:         models.TaskTypeVideo,
		Nonce:            "0x01",
		MinVram:          &minVRAM,
		RequiredGPU:      &darwinGPU,
		RequiredGPUVram:  &gpuVRAM,
		TaskFee:          models.BigInt{Int: *big.NewInt(100)},
	}
	ts, sig := signCanonicalPayload(t, videoInput)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/inference_tasks/"+videoInput.TaskIDCommitment, nil)
	_, err := CreateTask(ctx, &TaskInputWithSignature{TaskInput: videoInput, Timestamp: ts, Signature: sig})
	var valErr *response.ValidationErrorResponse
	if !errors.As(err, &valErr) || valErr.GetFieldName() != "required_gpu" {
		t.Fatalf("expected required_gpu ValidationErrorResponse for video + Darwin required_gpu, got %v", err)
	}
	if hfCalls != 0 {
		t.Fatalf("expected early platform rejection before Hub calls, got %d Hub calls", hfCalls)
	}

	// 2. SD task with Hub-detected Nunchaku (RequireLinuxCuda=true) pinning Windows RequiredGPU is rejected with validation_error on required_gpu.
	sdWinVRAM := uint64(24)
	sdNunchakuInput := TaskInput{
		TaskIDCommitment: "0x2222222222222222222222222222222222222222222222222222222222222222",
		TaskArgs:         `{"version":"3.8.0","base_model":{"name":"org/nunchaku-sd"},"prompt":"cat","task_config":{"num_images":1,"steps":4}}`,
		TaskType:         models.TaskTypeSD,
		Nonce:            "0x02",
		MinVram:          &minVRAM,
		RequiredGPU:      &windowsGPU,
		RequiredGPUVram:  &sdWinVRAM,
		TaskFee:          models.BigInt{Int: *big.NewInt(100)},
	}
	ts2, sig2 := signCanonicalPayload(t, sdNunchakuInput)
	ctx2, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx2.Request = httptest.NewRequest(http.MethodPost, "/v1/inference_tasks/"+sdNunchakuInput.TaskIDCommitment, nil)
	_, err = CreateTask(ctx2, &TaskInputWithSignature{TaskInput: sdNunchakuInput, Timestamp: ts2, Signature: sig2})
	valErr = nil
	if !errors.As(err, &valErr) || valErr.GetFieldName() != "required_gpu" {
		t.Fatalf("expected required_gpu ValidationErrorResponse for RequireLinuxCuda SD + Windows required_gpu, got %v", err)
	}

	// 3. Batch create returns permanent_error for both items and persists zero tasks.
	batchIn := BatchCreateInput{Items: []TaskInput{videoInput, sdNunchakuInput}}
	batchTS, batchSig := signCanonicalPayload(t, batchIn)
	batchCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	batchCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/inference_tasks/batch", nil)
	batchResp, err := CreateTaskBatch(batchCtx, &BatchCreateInputWithSignature{
		BatchCreateInput: batchIn,
		Timestamp:        batchTS,
		Signature:        batchSig,
	})
	if err != nil {
		t.Fatalf("unexpected whole-batch error: %v", err)
	}
	if len(batchResp.Data) != 2 {
		t.Fatalf("expected 2 batch item results, got %d", len(batchResp.Data))
	}
	for i, item := range batchResp.Data {
		if item.Outcome != "permanent_error" || !strings.Contains(item.Error, "unsupported required_gpu platform") {
			t.Fatalf("item %d: expected permanent_error with unsupported required_gpu platform, got %+v", i, item)
		}
	}
	var taskCount int64
	if err := config.GetDB().Model(&models.InferenceTask{}).Count(&taskCount).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected 0 created tasks, got %d", taskCount)
	}
}
