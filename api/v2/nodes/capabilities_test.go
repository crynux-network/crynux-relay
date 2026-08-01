package nodes

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const nodeCapabilitiesTestPrivateKey = "420fcabfd5dbb55215490693062e6e530840c64de837d071f0d9da21aaac861e"

func initNodeCapabilitiesAPITest(t *testing.T) {
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
		"  tracing_max_task_events: 50\n" +
		"withdraw:\n" +
		"  max_withdrawals_per_day: 10\n"
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
		&models.Node{},
		&models.NodeModel{},
		&models.NodeNameCount{},
		&models.NetworkNodeData{},
	); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}
}

func signedNodeCapabilitiesInput(t *testing.T) *NodeCapabilitiesInputWithSignature {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(nodeCapabilitiesTestPrivateKey)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	input := NodeCapabilitiesInput{
		Address:  crypto.PubkeyToAddress(privateKey.PublicKey).Hex(),
		GPUName:  "RTX 4090+docker",
		GPUVram:  24,
		Version:  "2.1.0",
		ModelIDs: []string{"base:model-new"},
	}
	timestamp := time.Now().Unix()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	var canonical interface{}
	if err := json.Unmarshal(data, &canonical); err != nil {
		t.Fatalf("failed to canonicalize input: %v", err)
	}
	data, err = json.Marshal(canonical)
	if err != nil {
		t.Fatalf("failed to marshal canonical input: %v", err)
	}
	hash := crypto.Keccak256Hash(append(data, []byte(strconv.FormatInt(timestamp, 10))...))
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("failed to sign input: %v", err)
	}
	return &NodeCapabilitiesInputWithSignature{
		NodeCapabilitiesInput: input,
		Timestamp:             timestamp,
		Signature:             hexutil.Encode(signature),
	}
}

func newNodeCapabilitiesTestContext(address string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v2/node/"+address+"/capabilities", nil)
	return c
}

func TestSyncNodeCapabilitiesAPIUpdatesJoinedNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initNodeCapabilitiesAPITest(t)
	input := signedNodeCapabilitiesInput(t)
	node := models.Node{
		Address:      input.Address,
		Network:      "base",
		Status:       models.NodeStatusAvailable,
		GPUName:      "RTX 3090+docker",
		GPUVram:      24,
		MajorVersion: 1,
	}
	if err := config.GetDB().Create(&node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	if err := config.GetDB().Create(&models.NetworkNodeData{
		Address: node.Address, Network: node.Network, CardModel: node.GPUName, VRam: int(node.GPUVram),
	}).Error; err != nil {
		t.Fatalf("failed to create network node data: %v", err)
	}
	if err := config.GetDB().Transaction(func(tx *gorm.DB) error {
		return service.IncrementNodeNameCountTx(context.Background(), tx, &node)
	}); err != nil {
		t.Fatalf("failed to create node name count: %v", err)
	}

	c := newNodeCapabilitiesTestContext(input.Address)
	response, err := SyncNodeCapabilities(c, input)
	if err != nil {
		t.Fatalf("capability sync API failed: %v", err)
	}
	if response == nil {
		t.Fatal("expected response")
	}
	updated, err := models.GetNodeWithModelsByAddress(c.Request.Context(), config.GetDB(), node.Address)
	if err != nil {
		t.Fatalf("failed to load updated node: %v", err)
	}
	if updated.GPUName != input.GPUName || updated.MajorVersion != 2 || len(updated.Models) != 1 {
		t.Fatalf("unexpected updated node: %#v", updated)
	}
}

func TestSyncNodeCapabilitiesAPIRejectsQuitNode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initNodeCapabilitiesAPITest(t)
	input := signedNodeCapabilitiesInput(t)
	if err := config.GetDB().Create(&models.Node{
		Address: input.Address,
		Status:  models.NodeStatusQuit,
	}).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	c := newNodeCapabilitiesTestContext(input.Address)
	if _, err := SyncNodeCapabilities(c, input); err == nil {
		t.Fatal("expected quit node capability sync to fail")
	}
}
