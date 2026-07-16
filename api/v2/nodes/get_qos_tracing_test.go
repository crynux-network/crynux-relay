package nodes

import (
	"crynux_relay/api/tools"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
)

func initNodeQosTracingTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	content := "environment: test\n" +
		"blockchains: {}\n" +
		"http:\n" +
		"  max_body_bytes: 33554432\n" +
		"  jwt:\n" +
		"    expires_in: 3600\n" +
		"stats:\n" +
		"  init_start_time: \"2026-01-01T00:00:00Z\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"task_pricing:\n" +
		"  overhead_seconds: 30\n" +
		"  initial_seconds_per_sd_unit: 10\n" +
		"  initial_seconds_per_llm_token: 0.1\n" +
		"  calibration_alpha: 0.1\n" +
		"  default_llm_max_new_tokens: 256\n" +
		"  base_vram: 8\n" +
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
		"  tracing_max_task_events: 3\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	tools.InitializeJWTManager()
}

func newQosTracingTestContext(rawURL string) *gin.Context {
	req := httptest.NewRequest(http.MethodGet, rawURL, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func TestGetNodeQosTracingAllowsJWTAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initNodeQosTracingTestConfig(t)
	address := "0x0000000000000000000000000000000000000a01"
	service.RecordNodeQosTrace(service.NodeQosTraceInput{
		NodeAddress: address,
		EventType:   service.QosTraceEventTaskTimeoutPenalty,
		Before:      service.NodeQosTraceValues{QosLong: 0.5, QosShort: 1.0, Qos: 0.5},
		After:       service.NodeQosTraceValues{QosLong: 0.5, QosShort: 0.95, Qos: 0.475},
	})
	token, _, err := tools.GenerateToken(address)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	c := newQosTracingTestContext("/v2/node/" + address + "/qos/tracing")
	c.Request.Header.Set("Authorization", "Bearer "+token)

	res, err := GetNodeQosTracing(c, &GetNodeInputWithSignature{GetNodeInput: GetNodeInput{Address: address}})
	if err != nil {
		t.Fatalf("expected jwt authorization, got error: %v", err)
	}
	if res.Data.NodeAddress != address || res.Data.MaxTaskEvents != 3 {
		t.Fatalf("unexpected response data: %+v", res.Data)
	}
	if len(res.Data.Events) != 1 || res.Data.Events[0].EventType != service.QosTraceEventTaskTimeoutPenalty {
		t.Fatalf("unexpected trace events: %+v", res.Data.Events)
	}
}

func TestGetNodeQosTracingAllowsSignatureAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initNodeQosTracingTestConfig(t)
	privateKeyHex := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	timestamp := time.Now().Unix()
	signature := signGetNodeInput(t, privateKeyHex, GetNodeInput{Address: address}, timestamp)

	res, err := GetNodeQosTracing(
		newQosTracingTestContext("/v2/node/"+address+"/qos/tracing"),
		&GetNodeInputWithSignature{
			GetNodeInput: GetNodeInput{Address: address},
			Timestamp:    &timestamp,
			Signature:    signature,
		},
	)
	if err != nil {
		t.Fatalf("expected signature authorization, got error: %v", err)
	}
	if res.Data.NodeAddress != address {
		t.Fatalf("unexpected node address %s", res.Data.NodeAddress)
	}
}

func TestGetNodeQosTracingRejectsMismatchedJWTAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initNodeQosTracingTestConfig(t)
	token, _, err := tools.GenerateToken("0x0000000000000000000000000000000000000b01")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	c := newQosTracingTestContext("/v2/node/0x0000000000000000000000000000000000000b02/qos/tracing")
	c.Request.Header.Set("Authorization", "Bearer "+token)

	_, err = GetNodeQosTracing(c, &GetNodeInputWithSignature{GetNodeInput: GetNodeInput{Address: "0x0000000000000000000000000000000000000b02"}})
	if validationErr, ok := err.(*response.ValidationErrorResponse); !ok || validationErr.GetFieldName() != "address" {
		t.Fatalf("expected address validation error, got %v", err)
	}
}

func TestGetNodeQosTracingReturnsEmptyEventList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initNodeQosTracingTestConfig(t)
	address := "0x0000000000000000000000000000000000000c01"
	token, _, err := tools.GenerateToken(address)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	c := newQosTracingTestContext("/v2/node/" + address + "/qos/tracing")
	c.Request.Header.Set("Authorization", "Bearer "+token)

	res, err := GetNodeQosTracing(c, &GetNodeInputWithSignature{GetNodeInput: GetNodeInput{Address: address}})
	if err != nil {
		t.Fatalf("expected empty authorized response, got error: %v", err)
	}
	if len(res.Data.Events) != 0 {
		t.Fatalf("expected empty event list, got %+v", res.Data.Events)
	}
}

func signGetNodeInput(t *testing.T, privateKeyHex string, input GetNodeInput, timestamp int64) string {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	dataBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	signBytes := append(dataBytes, []byte(strconv.FormatInt(timestamp, 10))...)
	dataHash := crypto.Keccak256Hash(signBytes)
	signature, err := crypto.Sign(dataHash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("failed to sign input: %v", err)
	}
	return hexutil.Encode(signature)
}
