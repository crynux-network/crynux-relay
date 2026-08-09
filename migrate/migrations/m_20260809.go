package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskLLMWorkloadForM20260809 struct {
	LLMTextInputBytes *uint64 `gorm:"type:bigint unsigned;null;default:null"`
	LLMImageCount     *uint64 `gorm:"type:bigint unsigned;null;default:null"`
	LLMImagePixels    *uint64 `gorm:"type:bigint unsigned;null;default:null"`
}

func (inferenceTaskLLMWorkloadForM20260809) TableName() string {
	return "inference_tasks"
}

type gpuExecutionCalibrationLLMForM20260809 struct {
	LLMModelSwitchSeconds  float64 `gorm:"not null;default:0"`
	LLMSecondsPerImage     float64 `gorm:"not null;default:0"`
	LLMSecondsPerMegapixel float64 `gorm:"not null;default:0"`
	LLMFormulaVersion      uint64  `gorm:"not null;default:0"`
	LLMXTX03               float64 `gorm:"column:llm_xtx_03;not null;default:0"`
	LLMXTX04               float64 `gorm:"column:llm_xtx_04;not null;default:0"`
	LLMXTX05               float64 `gorm:"column:llm_xtx_05;not null;default:0"`
	LLMXTX13               float64 `gorm:"column:llm_xtx_13;not null;default:0"`
	LLMXTX14               float64 `gorm:"column:llm_xtx_14;not null;default:0"`
	LLMXTX15               float64 `gorm:"column:llm_xtx_15;not null;default:0"`
	LLMXTX23               float64 `gorm:"column:llm_xtx_23;not null;default:0"`
	LLMXTX24               float64 `gorm:"column:llm_xtx_24;not null;default:0"`
	LLMXTX25               float64 `gorm:"column:llm_xtx_25;not null;default:0"`
	LLMXTX33               float64 `gorm:"column:llm_xtx_33;not null;default:0"`
	LLMXTX34               float64 `gorm:"column:llm_xtx_34;not null;default:0"`
	LLMXTX35               float64 `gorm:"column:llm_xtx_35;not null;default:0"`
	LLMXTX44               float64 `gorm:"column:llm_xtx_44;not null;default:0"`
	LLMXTX45               float64 `gorm:"column:llm_xtx_45;not null;default:0"`
	LLMXTX55               float64 `gorm:"column:llm_xtx_55;not null;default:0"`
	LLMXTY3                float64 `gorm:"column:llm_xty_3;not null;default:0"`
	LLMXTY4                float64 `gorm:"column:llm_xty_4;not null;default:0"`
	LLMXTY5                float64 `gorm:"column:llm_xty_5;not null;default:0"`
}

func (gpuExecutionCalibrationLLMForM20260809) TableName() string {
	return "gpu_execution_calibrations"
}

var inferenceTaskLLMWorkloadColumnsForM20260809 = []string{
	"LLMTextInputBytes", "LLMImageCount", "LLMImagePixels",
}

var gpuExecutionCalibrationLLMColumnsForM20260809 = []string{
	"LLMModelSwitchSeconds", "LLMSecondsPerImage", "LLMSecondsPerMegapixel", "LLMFormulaVersion",
	"LLMXTX03", "LLMXTX04", "LLMXTX05",
	"LLMXTX13", "LLMXTX14", "LLMXTX15",
	"LLMXTX23", "LLMXTX24", "LLMXTX25",
	"LLMXTX33", "LLMXTX34", "LLMXTX35",
	"LLMXTX44", "LLMXTX45", "LLMXTX55",
	"LLMXTY3", "LLMXTY4", "LLMXTY5",
}

func M20260809(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260809",
			Migrate: func(tx *gorm.DB) error {
				for _, column := range inferenceTaskLLMWorkloadColumnsForM20260809 {
					if err := tx.Migrator().AddColumn(&inferenceTaskLLMWorkloadForM20260809{}, column); err != nil {
						return err
					}
				}
				for _, column := range gpuExecutionCalibrationLLMColumnsForM20260809 {
					if err := tx.Migrator().AddColumn(&gpuExecutionCalibrationLLMForM20260809{}, column); err != nil {
						return err
					}
				}
				return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Table("gpu_execution_calibrations").Updates(map[string]interface{}{
					"llm_constant_seconds":         0,
					"llm_seconds_per_input_byte":   0,
					"llm_seconds_per_output_token": 0,
					"llm_model_switch_seconds":     0,
					"llm_seconds_per_image":        0,
					"llm_seconds_per_megapixel":    0,
					"llm_formula_version":          0,
					"llm_xtx_00":                   0,
					"llm_xtx_01":                   0,
					"llm_xtx_02":                   0,
					"llm_xtx_03":                   0,
					"llm_xtx_04":                   0,
					"llm_xtx_05":                   0,
					"llm_xtx_11":                   0,
					"llm_xtx_12":                   0,
					"llm_xtx_13":                   0,
					"llm_xtx_14":                   0,
					"llm_xtx_15":                   0,
					"llm_xtx_22":                   0,
					"llm_xtx_23":                   0,
					"llm_xtx_24":                   0,
					"llm_xtx_25":                   0,
					"llm_xtx_33":                   0,
					"llm_xtx_34":                   0,
					"llm_xtx_35":                   0,
					"llm_xtx_44":                   0,
					"llm_xtx_45":                   0,
					"llm_xtx_55":                   0,
					"llm_xty_0":                    0,
					"llm_xty_1":                    0,
					"llm_xty_2":                    0,
					"llm_xty_3":                    0,
					"llm_xty_4":                    0,
					"llm_xty_5":                    0,
					"llm_success_samples":          0,
				}).Error
			},
			Rollback: func(tx *gorm.DB) error {
				for i := len(gpuExecutionCalibrationLLMColumnsForM20260809) - 1; i >= 0; i-- {
					if err := tx.Migrator().DropColumn(&gpuExecutionCalibrationLLMForM20260809{}, gpuExecutionCalibrationLLMColumnsForM20260809[i]); err != nil {
						return err
					}
				}
				for i := len(inferenceTaskLLMWorkloadColumnsForM20260809) - 1; i >= 0; i-- {
					if err := tx.Migrator().DropColumn(&inferenceTaskLLMWorkloadForM20260809{}, inferenceTaskLLMWorkloadColumnsForM20260809[i]); err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
}
