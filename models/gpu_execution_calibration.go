package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// GPUExecutionCalibration stores execution parameters for one exact GPU variant.
type GPUExecutionCalibration struct {
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

func LoadGPUExecutionCalibrations(ctx context.Context, db *gorm.DB) ([]GPUExecutionCalibration, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var records []GPUExecutionCalibration
	if err := db.WithContext(dbCtx).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
