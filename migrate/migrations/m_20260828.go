package migrations

import (
	"crynux_relay/config"
	"database/sql"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskBatchFieldsForM20260828 struct {
	ID                      uint
	Status                  uint8
	TaskIDCommitment        string
	ExecutionGPU            string       `gorm:"column:execution_gpu;size:191;not null;default:''"`
	ExecutionGPUVRAM        uint64       `gorm:"column:execution_gpu_vram;not null;default:0"`
	EstimatedCompletionTime sql.NullTime `gorm:"null;default:null"`
	DeadlineAt              sql.NullTime `gorm:"null;default:null"`
	CreateTime              sql.NullTime
	StartTime               sql.NullTime
	ScoreReadyTime          sql.NullTime
	ValidatedTime           sql.NullTime
	Timeout                 uint64
	TaskType                uint8
}

func (inferenceTaskBatchFieldsForM20260828) TableName() string {
	return "inference_tasks"
}

const (
	taskStatusQueuedForM20260828             uint8 = 0
	taskStatusStartedForM20260828            uint8 = 1
	taskStatusParametersUploadedForM20260828 uint8 = 2
	taskStatusErrorReportedForM20260828      uint8 = 3
	taskStatusScoreReadyForM20260828         uint8 = 4
	taskStatusValidatedForM20260828          uint8 = 5
	taskStatusGroupValidatedForM20260828     uint8 = 6

	taskTypeSDForM20260828       uint8 = 0
	taskTypeLLMForM20260828      uint8 = 1
	taskTypeSDFTLoraForM20260828 uint8 = 2

	taskDeadlineBackfillBatchSizeForM20260828 = 500
)

func M20260828(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20260828",
		Migrate: func(tx *gorm.DB) error {
			model := &inferenceTaskBatchFieldsForM20260828{}
			for _, column := range []string{"ExecutionGPU", "ExecutionGPUVRAM", "EstimatedCompletionTime", "DeadlineAt"} {
				if err := tx.Migrator().AddColumn(model, column); err != nil {
					return err
				}
			}
			if err := tx.Exec("CREATE INDEX idx_inference_tasks_status_deadline_id ON inference_tasks (status, deadline_at, id)").Error; err != nil {
				return err
			}
			if err := backfillTaskDeadlinesForM20260828(tx); err != nil {
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropIndex(&inferenceTaskBatchFieldsForM20260828{}, "idx_inference_tasks_status_deadline_id"); err != nil {
				return err
			}
			for _, column := range []string{"DeadlineAt", "EstimatedCompletionTime", "ExecutionGPUVRAM", "ExecutionGPU"} {
				if err := tx.Migrator().DropColumn(&inferenceTaskBatchFieldsForM20260828{}, column); err != nil {
					return err
				}
			}
			return nil
		},
	}})
}

func backfillTaskDeadlinesForM20260828(tx *gorm.DB) error {
	for status := taskStatusQueuedForM20260828; status <= taskStatusGroupValidatedForM20260828; status++ {
		var lastID uint
		for {
			var tasks []inferenceTaskBatchFieldsForM20260828
			query := tx.
				Where("status = ? AND deadline_at IS NULL", status).
				Order("id ASC").
				Limit(taskDeadlineBackfillBatchSizeForM20260828)
			if lastID > 0 {
				query = query.Where("id > ?", lastID)
			}
			if err := query.Find(&tasks).Error; err != nil {
				return err
			}
			if len(tasks) == 0 {
				break
			}
			for i := range tasks {
				task := &tasks[i]
				deadline, ok := deadlineAtForM20260828(task)
				if !ok {
					continue
				}
				if err := tx.Model(modelForTaskIDM20260828(task.ID)).Update("deadline_at", deadline).Error; err != nil {
					return err
				}
			}
			lastID = tasks[len(tasks)-1].ID
			if len(tasks) < taskDeadlineBackfillBatchSizeForM20260828 {
				break
			}
		}
	}
	return nil
}

func deadlineAtForM20260828(task *inferenceTaskBatchFieldsForM20260828) (time.Time, bool) {
	relayOwned := task.TaskType == taskTypeSDForM20260828 || task.TaskType == taskTypeLLMForM20260828
	if !relayOwned && task.TaskType != taskTypeSDFTLoraForM20260828 {
		return time.Time{}, false
	}
	switch task.Status {
	case taskStatusQueuedForM20260828:
		if !task.CreateTime.Valid {
			return time.Time{}, false
		}
		if relayOwned {
			return task.CreateTime.Time.Add(time.Duration(config.GetConfig().TaskPricing.QueueTimeoutSeconds) * time.Second), true
		}
		return task.CreateTime.Time.Add(3*time.Minute + time.Duration(task.Timeout)*time.Second), true
	case taskStatusStartedForM20260828, taskStatusParametersUploadedForM20260828:
		if !task.StartTime.Valid {
			return time.Time{}, false
		}
		return task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second), true
	case taskStatusErrorReportedForM20260828, taskStatusScoreReadyForM20260828:
		if relayOwned && task.ScoreReadyTime.Valid {
			return task.ScoreReadyTime.Time.Add(time.Duration(config.GetConfig().TaskPricing.AppValidationTimeoutSeconds) * time.Second), true
		}
		if task.StartTime.Valid {
			return task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second), true
		}
		return time.Time{}, false
	case taskStatusValidatedForM20260828, taskStatusGroupValidatedForM20260828:
		if relayOwned && task.ValidatedTime.Valid {
			return task.ValidatedTime.Time.Add(time.Duration(config.GetConfig().TaskPricing.ResultUploadTimeoutSeconds) * time.Second), true
		}
		if task.StartTime.Valid {
			return task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second), true
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}

func modelForTaskIDM20260828(id uint) *inferenceTaskBatchFieldsForM20260828 {
	return &inferenceTaskBatchFieldsForM20260828{ID: id}
}
