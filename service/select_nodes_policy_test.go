package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func initSelectionPolicyTestConfig(t *testing.T, minCount uint64, whitelistEnabled bool) {
	t.Helper()
	dir := t.TempDir()
	whitelistFlag := "false"
	if whitelistEnabled {
		whitelistFlag = "true"
	}
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
		"  minimum_node_name_number: " + strconv.FormatUint(minCount, 10) + "\n" +
		"  node_name_whitelist_enabled: " + whitelistFlag + "\n" +
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

func TestFilterNodesByNodeNamePolicy(t *testing.T) {
	initSelectionPolicyTestConfig(t, 2, true)
	resetNodeNamePolicyCacheForTest()

	db := config.GetDB()
	if err := db.AutoMigrate(&models.NodeNameWhitelist{}, &models.NodeNameCount{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	ctx := context.Background()

	if err := AddNodeNameWhitelist(ctx, db, "A100", 40, "1.0.0"); err != nil {
		t.Fatalf("failed to add whitelist entry: %v", err)
	}
	if err := db.Create(&models.NodeNameCount{
		GPUName:     "A100",
		GPUVram:     40,
		NodeVersion: "1.0.0",
		ActiveCount: 2,
	}).Error; err != nil {
		t.Fatalf("failed to seed count entry: %v", err)
	}
	if err := db.Create(&models.NodeNameCount{
		GPUName:     "A100",
		GPUVram:     40,
		NodeVersion: "2.0.0",
		ActiveCount: 100,
	}).Error; err != nil {
		t.Fatalf("failed to seed non-whitelisted count entry: %v", err)
	}
	if err := db.Create(&models.NodeNameCount{
		GPUName:     "L40",
		GPUVram:     24,
		NodeVersion: "1.0.0",
		ActiveCount: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed below-minimum count entry: %v", err)
	}
	if err := RefreshNodeNameCountCache(ctx, db); err != nil {
		t.Fatalf("failed to refresh count cache: %v", err)
	}

	nodes := []models.Node{
		{GPUName: "A100", GPUVram: 40, MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
		{GPUName: "A100", GPUVram: 40, MajorVersion: 2, MinorVersion: 0, PatchVersion: 0},
		{GPUName: "L40", GPUVram: 24, MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
	}
	filtered, err := filterNodesByNodeNamePolicy(ctx, nodes)
	if err != nil {
		t.Fatalf("filter should succeed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("unexpected filtered size: %d", len(filtered))
	}
	if filtered[0].GPUName != "A100" || filtered[0].MajorVersion != 1 {
		t.Fatalf("unexpected filtered node: %#v", filtered[0])
	}
}

func TestBuildEntryTraceCandidatePoolUsesFinalWeights(t *testing.T) {
	entries := []*NodeIndexEntry{
		{Address: "0xnode1", GPUName: "RTX 4090"},
		{Address: "0xnode2", GPUName: "A100"},
	}
	probs := []NodeSelectingProb{
		{StakingScore: 0.5, QOSScore: 0.8, ProbWeight: 0.31},
		{StakingScore: 1, QOSScore: 0.9, ProbWeight: 0.47},
	}
	finalWeights := []float64{0.62, 0.47}

	candidatePool, totalCount, truncated := buildEntryTraceCandidatePool(entries, probs, finalWeights)

	if totalCount != 2 {
		t.Fatalf("expected total count 2, got %d", totalCount)
	}
	if truncated {
		t.Fatal("expected candidate pool not to be truncated")
	}
	if len(candidatePool) != 2 {
		t.Fatalf("expected two candidate pool records, got %d", len(candidatePool))
	}
	if candidatePool[0].ProbWeight != 0.62 {
		t.Fatalf("expected final prob weight 0.62, got %f", candidatePool[0].ProbWeight)
	}
	if candidatePool[0].StakingScore != 0.5 || candidatePool[0].QOSScore != 0.8 {
		t.Fatalf("expected staking and QoS scores from base probability inputs, got %#v", candidatePool[0])
	}
}
