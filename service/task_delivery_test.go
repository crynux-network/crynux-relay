package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func initDeliveryTestConfig(t *testing.T) {
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

func TestMarkTaskDeliveredSetsDeliveredTimeOnce(t *testing.T) {
	initDeliveryTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	ctx := context.Background()

	task := &models.InferenceTask{
		TaskIDCommitment: "0x01",
		SelectedNode:     "0xnode",
	}
	if err := task.Create(ctx, db); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if err := MarkTaskDelivered(ctx, db, task); err != nil {
		t.Fatalf("failed to mark task delivered: %v", err)
	}
	if !task.DeliveredTime.Valid {
		t.Fatalf("expected delivered time to be set")
	}
	firstDeliveredTime := task.DeliveredTime.Time

	var stored models.InferenceTask
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	if !stored.DeliveredTime.Valid {
		t.Fatalf("expected stored delivered time to be set")
	}

	if err := MarkTaskDelivered(ctx, db, task); err != nil {
		t.Fatalf("failed to mark task delivered again: %v", err)
	}
	if !task.DeliveredTime.Time.Equal(firstDeliveredTime) {
		t.Fatalf("expected delivered time to stay unchanged on repeated delivery")
	}

	reloaded := &models.InferenceTask{}
	if err := db.First(reloaded, task.ID).Error; err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if err := MarkTaskDelivered(ctx, db, reloaded); err != nil {
		t.Fatalf("failed to mark already delivered task: %v", err)
	}
	if !reloaded.DeliveredTime.Time.Equal(stored.DeliveredTime.Time) {
		t.Fatalf("expected conditional update to keep the first delivered time")
	}
}

func TestTouchNodeLastSeenWritesAndThrottles(t *testing.T) {
	initDeliveryTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	ctx := context.Background()

	node := &models.Node{Address: "0xlastseen"}
	if err := node.Save(ctx, db); err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := TouchNodeLastSeen(ctx, db, node.Address); err != nil {
		t.Fatalf("failed to touch node last seen: %v", err)
	}
	var stored models.Node
	if err := db.Where("address = ?", node.Address).First(&stored).Error; err != nil {
		t.Fatalf("failed to load node: %v", err)
	}
	if !stored.LastSeenTime.Valid {
		t.Fatalf("expected last seen time to be set")
	}
	firstSeen := stored.LastSeenTime.Time

	time.Sleep(10 * time.Millisecond)
	if err := TouchNodeLastSeen(ctx, db, node.Address); err != nil {
		t.Fatalf("failed to touch node last seen again: %v", err)
	}
	if err := db.Where("address = ?", node.Address).First(&stored).Error; err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if !stored.LastSeenTime.Time.Equal(firstSeen) {
		t.Fatalf("expected throttled touch to skip the DB write")
	}
}
