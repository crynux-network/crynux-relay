package models

import (
	"time"

	"gorm.io/gorm"
)

type NodeIncentive struct {
	gorm.Model
	NodeAddress string `gorm:"index"`
	Incentive   float64
	Time        time.Time `gorm:"index"`
	TaskCount   int64
	SDTaskCount       int64 `gorm:"column:sd_task_count"`
	LLMTaskCount      int64 `gorm:"column:llm_task_count"`
	SDFTLoraTaskCount int64 `gorm:"column:sd_ft_lora_task_count"`
	VideoTaskCount    int64 `gorm:"column:video_task_count;not null;default:0"`
}
