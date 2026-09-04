package service

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func initQosTestConfig(t *testing.T) {
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
		"  score_pool_size: 50\n" +
		"  tracing_max_task_events: 50\n" +
		"  kickout_threshold: 2.0\n" +
		"  rejoin_qos_long_floor: 0.3\n" +
		qosHealthTestConfigYAML
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

func TestCalculateLongTermQosNormalizesPersistedScore(t *testing.T) {
	if qosLong := CalculateLongTermQos(0); qosLong != 0 {
		t.Fatalf("expected zero persisted score to normalize to 0, got %f", qosLong)
	}
	if qosLong := CalculateLongTermQos(5); qosLong != 0.5 {
		t.Fatalf("expected initial persisted score to normalize to 0.5, got %f", qosLong)
	}
}

func TestNodeTaskQosScoreReplacesInitialScores(t *testing.T) {
	initQosTestConfig(t)

	const address = "0xnew"
	defer resetNodeQosScorePool(address)
	node := &models.Node{Address: address, QOSScore: 5.0}

	firstScore, err := getNodeTaskQosScore(node, 10)
	if err != nil {
		t.Fatalf("failed to get first node task qos score: %v", err)
	}
	poolSize := config.GetConfig().QoS.ScorePoolSize
	expectedFirst := (float64(poolSize-1)*5 + 10) / float64(poolSize)
	if firstScore != expectedFirst {
		t.Fatalf("expected first score %.2f, got %.2f", expectedFirst, firstScore)
	}
	if size := getNodeQosWindowSize(address); size != int(poolSize) {
		t.Fatalf("expected initialized pool size %d, got %d", poolSize, size)
	}

	secondScore, err := getNodeTaskQosScore(node, 2)
	if err != nil {
		t.Fatalf("failed to get second node task qos score: %v", err)
	}
	expectedSecond := (float64(poolSize-2)*5 + 10 + 2) / float64(poolSize)
	if secondScore != expectedSecond {
		t.Fatalf("expected second score %.2f, got %.2f", expectedSecond, secondScore)
	}
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

func TestAdjustZeroQosNodeForJoinRebuildsFromFloor(t *testing.T) {
	initQosTestConfig(t)

	const address = "0xzero-rejoin"
	poolSize := config.GetConfig().QoS.ScorePoolSize
	seedNodeQosScorePool(address, 0, int(poolSize))
	defer resetNodeQosScorePool(address)

	node := &models.Node{Address: address, QOSScore: 0}
	AdjustNodeQosForJoin(node, false)

	floorScore := config.GetConfig().QoS.RejoinQosLongFloor * GetMaxQosScore()
	if node.QOSScore != floorScore {
		t.Fatalf("expected zero score to recover to %.2f, got %.2f", floorScore, node.QOSScore)
	}
	if size := getNodeQosWindowSize(address); size != 0 {
		t.Fatalf("expected stale zero score pool to be reset, got %d entries", size)
	}

	newScore, err := getNodeTaskQosScore(node, 10)
	if err != nil {
		t.Fatalf("failed to get first post-rejoin score: %v", err)
	}
	expectedScore := (float64(poolSize-1)*floorScore + 10) / float64(poolSize)
	if newScore != expectedScore {
		t.Fatalf("expected rebuilt score %.2f, got %.2f", expectedScore, newScore)
	}
	node.QOSScore = newScore
	if ShouldPermanentKickout(node) {
		t.Fatal("expected rebuilt score pool not to trigger permanent kickout")
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

func TestApplyHealthPenaltySetsExclusionBelowEnterThreshold(t *testing.T) {
	initQosTestConfig(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("migrate node: %v", err)
	}
	now := time.Now().UTC()
	node := &models.Node{
		Address:         "0xpenalty",
		Status:          models.NodeStatusAvailable,
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: now, Valid: true},
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := ApplyHealthPenalty(t.Context(), db, node); err != nil {
		t.Fatalf("apply penalty: %v", err)
	}
	if !node.HealthExcluded {
		t.Fatal("expected node to be health excluded")
	}
	var persisted models.Node
	if err := db.First(&persisted, node.ID).Error; err != nil {
		t.Fatalf("read node: %v", err)
	}
	if !persisted.HealthExcluded {
		t.Fatal("expected exclusion to be persisted with health penalty")
	}
}

func TestApplyHealthPenaltyDoesNotExcludeAtEnterThreshold(t *testing.T) {
	initQosTestConfig(t)
	config.GetConfig().QoS.PenaltyFactor = 0.4
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("migrate node: %v", err)
	}
	node := &models.Node{
		Address:         "0xthreshold",
		Status:          models.NodeStatusAvailable,
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC().Add(time.Hour), Valid: true},
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := ApplyHealthPenalty(t.Context(), db, node); err != nil {
		t.Fatalf("apply penalty: %v", err)
	}
	if node.HealthBase != config.GetConfig().QoS.HealthExcludeEnterThreshold {
		t.Fatalf("expected health at enter threshold, got %f", node.HealthBase)
	}
	if node.HealthExcluded {
		t.Fatal("expected strict less-than exclusion semantics")
	}
}

func TestIsHealthExcludedUsesPersistedFlagAndExitThreshold(t *testing.T) {
	initQosTestConfig(t)
	now := time.Now().UTC()
	node := &models.Node{
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: now, Valid: true},
	}
	if IsHealthExcluded(node, now) {
		t.Fatal("expected an unset flag to remain eligible")
	}
	node.HealthExcluded = true
	if !IsHealthExcluded(node, now) {
		t.Fatal("expected exclusion below exit threshold")
	}
	node.HealthBase = config.GetConfig().QoS.HealthExcludeExitThreshold
	if IsHealthExcluded(node, now) {
		t.Fatal("expected health equal to exit threshold to be eligible")
	}
	node.HealthBase = 0.9
	if IsHealthExcluded(node, now) {
		t.Fatal("expected health above exit threshold to be eligible")
	}
}

func TestCalculateNodeSelectingProbZerosActiveExclusion(t *testing.T) {
	initMatchingTestConfig(t)
	node := models.Node{
		Address:         "0xexcluded",
		Status:          models.NodeStatusAvailable,
		StakeAmount:     models.BigInt{},
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	node.StakeAmount.SetInt64(1)
	if prob := CalculateNodeSelectingProb(node, time.Now().UTC()); prob.ProbWeight != 0 {
		t.Fatalf("expected zero selection weight, got %f", prob.ProbWeight)
	}
}

func TestShouldPermanentKickoutIgnoresShortTermHealth(t *testing.T) {
	initQosTestConfig(t)
	node := &models.Node{
		Address:         "0xshort-health",
		QOSScore:        5,
		HealthBase:      0,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if ShouldPermanentKickout(node) {
		t.Fatal("expected short-term health not to cause permanent kickout")
	}

	seedNodeQosScorePool(node.Address, 1, int(config.GetConfig().QoS.ScorePoolSize))
	defer resetNodeQosScorePool(node.Address)
	node.QOSScore = 1
	if !ShouldPermanentKickout(node) {
		t.Fatal("expected long-term QoS kickout to remain active")
	}
}
