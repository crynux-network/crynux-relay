package migrations

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20260828AddsTaskBatchFieldsAndIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskBatchFieldsForM20260828{}); err != nil {
		t.Fatalf("create inference task table: %v", err)
	}
	for _, column := range []string{"execution_gpu", "execution_gpu_vram", "estimated_completion_time", "deadline_at"} {
		if err := db.Migrator().DropColumn(&inferenceTaskBatchFieldsForM20260828{}, column); err != nil {
			t.Fatalf("drop preexisting column %s: %v", column, err)
		}
	}

	startTime := time.Now().Truncate(time.Second)
	tasks := make([]map[string]interface{}, taskDeadlineBackfillBatchSizeForM20260828+1)
	for i := range tasks {
		tasks[i] = map[string]interface{}{
			"status":             taskStatusStartedForM20260828,
			"task_id_commitment": fmt.Sprintf("active-%d", i),
			"start_time":         startTime,
			"timeout":            60,
			"task_type":          0,
		}
	}
	terminalTask := map[string]interface{}{
		"status":             taskStatusGroupValidatedForM20260828 + 1,
		"task_id_commitment": "terminal",
		"start_time":         startTime,
		"timeout":            60,
		"task_type":          0,
	}
	sdftTask := map[string]interface{}{
		"status":             taskStatusScoreReadyForM20260828,
		"task_id_commitment": "sdft",
		"start_time":         startTime,
		"score_ready_time":   startTime.Add(time.Minute),
		"timeout":            120,
		"task_type":          taskTypeSDFTLoraForM20260828,
	}
	if err := db.Table("inference_tasks").Create(&tasks).Error; err != nil {
		t.Fatalf("create active tasks: %v", err)
	}
	if err := db.Table("inference_tasks").Create(&terminalTask).Error; err != nil {
		t.Fatalf("create terminal task: %v", err)
	}
	if err := db.Table("inference_tasks").Create(&sdftTask).Error; err != nil {
		t.Fatalf("create SDFT task: %v", err)
	}

	migration := M20260828(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasIndex(&inferenceTaskBatchFieldsForM20260828{}, "idx_inference_tasks_status_deadline_id") {
		t.Fatal("missing deadline index")
	}

	var first, last, terminal, sdft inferenceTaskBatchFieldsForM20260828
	if err := db.Where("task_id_commitment = ?", "active-0").First(&first).Error; err != nil {
		t.Fatalf("read first active task: %v", err)
	}
	if err := db.Where("task_id_commitment = ?", fmt.Sprintf("active-%d", len(tasks)-1)).First(&last).Error; err != nil {
		t.Fatalf("read last active task: %v", err)
	}
	if err := db.Where("task_id_commitment = ?", "terminal").First(&terminal).Error; err != nil {
		t.Fatalf("read terminal task: %v", err)
	}
	if err := db.Where("task_id_commitment = ?", "sdft").First(&sdft).Error; err != nil {
		t.Fatalf("read SDFT task: %v", err)
	}
	wantDeadline := startTime.Add(60 * time.Second)
	if !first.DeadlineAt.Valid || !first.DeadlineAt.Time.Equal(wantDeadline) {
		t.Fatalf("first active deadline = %v, want %v", first.DeadlineAt, wantDeadline)
	}
	if !last.DeadlineAt.Valid || !last.DeadlineAt.Time.Equal(wantDeadline) {
		t.Fatalf("last active deadline = %v, want %v", last.DeadlineAt, wantDeadline)
	}
	if terminal.DeadlineAt.Valid {
		t.Fatalf("terminal deadline = %v, want NULL", terminal.DeadlineAt)
	}
	wantSDFTDeadline := startTime.Add(120 * time.Second)
	if !sdft.DeadlineAt.Valid || !sdft.DeadlineAt.Time.Equal(wantSDFTDeadline) {
		t.Fatalf("SDFT deadline = %v, want %v", sdft.DeadlineAt, wantSDFTDeadline)
	}
}
