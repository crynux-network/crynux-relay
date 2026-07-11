package service

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"os"
	"path/filepath"
	"testing"
)

func initQosTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	content := "environment: test\n" +
		"blockchains: {}\n" +
		"http:\n" +
		"  jwt:\n" +
		"    expires_in: 3600\n" +
		"stats:\n" +
		"  init_start_time: \"2026-01-01T00:00:00Z\"\n" +
		"task:\n" +
		"  passive_slash_mode: true\n" +
		"qos:\n" +
		"  score_pool_size: 50\n" +
		"  tracing_max_task_events: 50\n" +
		"  kickout_threshold: 2.0\n" +
		"  rejoin_qos_long_floor: 0.3\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err := config.InitConfig(dir); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
}

func seedNodeQosScorePool(address string, score uint64, count int) {
	pool := make([]uint64, count)
	for i := range pool {
		pool[i] = score
	}
	nodeQoSScorePool.mu.Lock()
	nodeQoSScorePool.pool[address] = pool
	nodeQoSScorePool.mu.Unlock()
}

func TestAdjustNodeQosForJoinResetsStaleScorePool(t *testing.T) {
	initQosTestConfig(t)

	const address = "0xrejoin"
	seedNodeQosScorePool(address, 1, 50)
	defer resetNodeQosScorePool(address)

	node := &models.Node{Address: address, QOSScore: 1.0}
	AdjustNodeQosForJoin(node, false)

	floorScore := config.GetConfig().QoS.RejoinQosLongFloor * GetMaxQosScore()
	if node.QOSScore != floorScore {
		t.Fatalf("expected rejoin floor score %.2f, got %.2f", floorScore, node.QOSScore)
	}
	if size := getNodeQosWindowSize(address); size != 0 {
		t.Fatalf("expected stale score pool to be reset, got %d entries", size)
	}

	newScore, err := getNodeTaskQosScore(node, 10)
	if err != nil {
		t.Fatalf("failed to get node task qos score: %v", err)
	}
	if newScore < config.GetConfig().QoS.KickoutThreshold {
		t.Fatalf("expected first post-rejoin score %.2f to stay above kickout threshold", newScore)
	}
}

func TestAdjustNodeQosForJoinKeepsPoolAboveFloor(t *testing.T) {
	initQosTestConfig(t)

	const address = "0xhealthy"
	seedNodeQosScorePool(address, 5, 50)
	defer resetNodeQosScorePool(address)

	node := &models.Node{Address: address, QOSScore: 5.0}
	AdjustNodeQosForJoin(node, false)

	if node.QOSScore != 5.0 {
		t.Fatalf("expected score to stay 5.0, got %.2f", node.QOSScore)
	}
	if size := getNodeQosWindowSize(address); size != 50 {
		t.Fatalf("expected score pool to be kept, got %d entries", size)
	}
}
