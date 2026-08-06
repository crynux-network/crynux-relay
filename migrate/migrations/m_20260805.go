package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskWorkloadForM20260805 struct {
	SDUnits       *uint64 `gorm:"type:bigint unsigned;null;default:null"`
	LLMInputBytes *uint64 `gorm:"type:bigint unsigned;null;default:null"`
}

func (inferenceTaskWorkloadForM20260805) TableName() string {
	return "inference_tasks"
}

type gpuExecutionCalibrationForM20260805 struct {
	ID                       uint    `gorm:"primaryKey"`
	GPUName                  string  `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_variant,priority:1"`
	GPUVram                  uint64  `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_variant,priority:2"`
	SecondsPerSDPixelStep    float64 `gorm:"not null"`
	SDSuccessSamples         uint64  `gorm:"not null"`
	LLMConstantSeconds       float64 `gorm:"not null"`
	LLMSecondsPerInputByte   float64 `gorm:"not null"`
	LLMSecondsPerOutputToken float64 `gorm:"not null"`
	LLMXTX00                 float64 `gorm:"column:llm_xtx_00;not null"`
	LLMXTX01                 float64 `gorm:"column:llm_xtx_01;not null"`
	LLMXTX02                 float64 `gorm:"column:llm_xtx_02;not null"`
	LLMXTX11                 float64 `gorm:"column:llm_xtx_11;not null"`
	LLMXTX12                 float64 `gorm:"column:llm_xtx_12;not null"`
	LLMXTX22                 float64 `gorm:"column:llm_xtx_22;not null"`
	LLMXTY0                  float64 `gorm:"column:llm_xty_0;not null"`
	LLMXTY1                  float64 `gorm:"column:llm_xty_1;not null"`
	LLMXTY2                  float64 `gorm:"column:llm_xty_2;not null"`
	LLMSuccessSamples        uint64  `gorm:"not null"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (gpuExecutionCalibrationForM20260805) TableName() string {
	return "gpu_execution_calibrations"
}

func M20260805(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260805",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().CreateTable(&gpuExecutionCalibrationForM20260805{}); err != nil {
					return err
				}
				if err := tx.Migrator().AddColumn(&inferenceTaskWorkloadForM20260805{}, "SDUnits"); err != nil {
					return err
				}
				return tx.Migrator().AddColumn(&inferenceTaskWorkloadForM20260805{}, "LLMInputBytes")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropColumn(&inferenceTaskWorkloadForM20260805{}, "LLMInputBytes"); err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&inferenceTaskWorkloadForM20260805{}, "SDUnits"); err != nil {
					return err
				}
				return tx.Migrator().DropTable(&gpuExecutionCalibrationForM20260805{})
			},
		},
	})
}
