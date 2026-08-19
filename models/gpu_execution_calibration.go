package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// GPUExecutionCalibration stores execution parameters for one GPU and model execution configuration.
type GPUExecutionCalibration struct {
	ID                       uint     `gorm:"primaryKey"`
	TaskType                 TaskType `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:1"`
	GPUName                  string   `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:2"`
	GPUVram                  uint64   `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:3"`
	ModelName                string   `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:4"`
	ModelVariant             string   `gorm:"size:191;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:5"`
	ExecutionDType           string   `gorm:"column:execution_dtype;size:64;not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:6"`
	QuantizeBits             uint64   `gorm:"not null;uniqueIndex:idx_gpu_execution_calibration_config,priority:7"`
	MinVRAMRequirement       uint64   `gorm:"column:min_vram_requirement;not null"`
	MaxVRAMRequirement       uint64   `gorm:"column:max_vram_requirement;not null"`
	SecondsPerSDPixelStep    float64  `gorm:"not null"`
	SDOverheadSeconds        float64  `gorm:"not null"`
	SDFormulaVersion         uint64   `gorm:"not null"`
	SDXTX00                  float64  `gorm:"column:sd_xtx_00;not null"`
	SDXTX01                  float64  `gorm:"column:sd_xtx_01;not null"`
	SDXTX11                  float64  `gorm:"column:sd_xtx_11;not null"`
	SDXTY0                   float64  `gorm:"column:sd_xty_0;not null"`
	SDXTY1                   float64  `gorm:"column:sd_xty_1;not null"`
	SDSuccessSamples         uint64   `gorm:"not null"`
	LLMConstantSeconds       float64  `gorm:"not null"`
	LLMSecondsPerInputByte   float64  `gorm:"not null"`
	LLMSecondsPerOutputToken float64  `gorm:"not null"`
	LLMModelSwitchSeconds    float64  `gorm:"not null"`
	LLMSecondsPerImage       float64  `gorm:"not null"`
	LLMSecondsPerMegapixel   float64  `gorm:"not null"`
	LLMFormulaVersion        uint64   `gorm:"not null"`
	LLMXTX00                 float64  `gorm:"column:llm_xtx_00;not null"`
	LLMXTX01                 float64  `gorm:"column:llm_xtx_01;not null"`
	LLMXTX02                 float64  `gorm:"column:llm_xtx_02;not null"`
	LLMXTX03                 float64  `gorm:"column:llm_xtx_03;not null"`
	LLMXTX04                 float64  `gorm:"column:llm_xtx_04;not null"`
	LLMXTX05                 float64  `gorm:"column:llm_xtx_05;not null"`
	LLMXTX11                 float64  `gorm:"column:llm_xtx_11;not null"`
	LLMXTX12                 float64  `gorm:"column:llm_xtx_12;not null"`
	LLMXTX13                 float64  `gorm:"column:llm_xtx_13;not null"`
	LLMXTX14                 float64  `gorm:"column:llm_xtx_14;not null"`
	LLMXTX15                 float64  `gorm:"column:llm_xtx_15;not null"`
	LLMXTX22                 float64  `gorm:"column:llm_xtx_22;not null"`
	LLMXTX23                 float64  `gorm:"column:llm_xtx_23;not null"`
	LLMXTX24                 float64  `gorm:"column:llm_xtx_24;not null"`
	LLMXTX25                 float64  `gorm:"column:llm_xtx_25;not null"`
	LLMXTX33                 float64  `gorm:"column:llm_xtx_33;not null"`
	LLMXTX34                 float64  `gorm:"column:llm_xtx_34;not null"`
	LLMXTX35                 float64  `gorm:"column:llm_xtx_35;not null"`
	LLMXTX44                 float64  `gorm:"column:llm_xtx_44;not null"`
	LLMXTX45                 float64  `gorm:"column:llm_xtx_45;not null"`
	LLMXTX55                 float64  `gorm:"column:llm_xtx_55;not null"`
	LLMXTY0                  float64  `gorm:"column:llm_xty_0;not null"`
	LLMXTY1                  float64  `gorm:"column:llm_xty_1;not null"`
	LLMXTY2                  float64  `gorm:"column:llm_xty_2;not null"`
	LLMXTY3                  float64  `gorm:"column:llm_xty_3;not null"`
	LLMXTY4                  float64  `gorm:"column:llm_xty_4;not null"`
	LLMXTY5                  float64  `gorm:"column:llm_xty_5;not null"`
	LLMSuccessSamples        uint64   `gorm:"not null"`
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
