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
	SDUnits                 *uint64
	LLMTextInputBytes       *uint64
	LLMImageCount           *uint64
	LLMImagePixels          *uint64
	LLMMaxNewTokens         *uint64
}

func (inferenceTaskBatchFieldsForM20260828) TableName() string {
	return "inference_tasks"
}

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
			if err := backfillTaskDeadlinesForM20260828(tx); err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX idx_inference_tasks_status_deadline_id ON inference_tasks (status, deadline_at, id)").Error; err != nil {
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
	var tasks []inferenceTaskBatchFieldsForM20260828
	if err := tx.Find(&tasks).Error; err != nil {
		return err
	}
	for i := range tasks {
		task := &tasks[i]
		var deadline time.Time
		relayOwned := (task.TaskType == 0 && task.SDUnits != nil) ||
			(task.TaskType == 1 && task.LLMTextInputBytes != nil && task.LLMImageCount != nil &&
				task.LLMImagePixels != nil && task.LLMMaxNewTokens != nil)
		switch task.Status {
		case 0:
			if !task.CreateTime.Valid {
				continue
			}
			if relayOwned {
				deadline = task.CreateTime.Time.Add(time.Duration(config.GetConfig().TaskPricing.QueueTimeoutSeconds) * time.Second)
			} else {
				deadline = task.CreateTime.Time.Add(3*time.Minute + time.Duration(task.Timeout)*time.Second)
			}
		case 1, 2:
			if !task.StartTime.Valid {
				continue
			}
			deadline = task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second)
		case 3, 4:
			if relayOwned && task.ScoreReadyTime.Valid {
				deadline = task.ScoreReadyTime.Time.Add(time.Duration(config.GetConfig().TaskPricing.AppValidationTimeoutSeconds) * time.Second)
			} else if task.StartTime.Valid {
				deadline = task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second)
			} else {
				continue
			}
		case 5, 6:
			if relayOwned && task.ValidatedTime.Valid {
				deadline = task.ValidatedTime.Time.Add(time.Duration(config.GetConfig().TaskPricing.ResultUploadTimeoutSeconds) * time.Second)
			} else if task.StartTime.Valid {
				deadline = task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second)
			} else {
				continue
			}
		default:
			continue
		}
		if err := tx.Model(modelForTaskIDM20260828(task.ID)).Update("deadline_at", deadline).Error; err != nil {
			return err
		}
	}
	return nil
}

func modelForTaskIDM20260828(id uint) *inferenceTaskBatchFieldsForM20260828 {
	return &inferenceTaskBatchFieldsForM20260828{ID: id}
}
