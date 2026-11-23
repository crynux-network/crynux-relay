package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskVideoUnitsForM20261118 struct {
	VideoUnits *uint64 `gorm:"column:video_units;type:bigint unsigned;null;default:null"`
}

func (inferenceTaskVideoUnitsForM20261118) TableName() string { return "inference_tasks" }

type gpuExecutionCalibrationVideoForM20261118 struct {
	SecondsPerVideoUnit  float64 `gorm:"column:seconds_per_video_unit;not null;default:0"`
	VideoOverheadSeconds float64 `gorm:"column:video_overhead_seconds;not null;default:0"`
	VideoFormulaVersion  uint64  `gorm:"column:video_formula_version;not null;default:0"`
	VideoXTX00           float64 `gorm:"column:video_xtx_00;not null;default:0"`
	VideoXTX01           float64 `gorm:"column:video_xtx_01;not null;default:0"`
	VideoXTX11           float64 `gorm:"column:video_xtx_11;not null;default:0"`
	VideoXTY0            float64 `gorm:"column:video_xty_0;not null;default:0"`
	VideoXTY1            float64 `gorm:"column:video_xty_1;not null;default:0"`
	VideoSuccessSamples  uint64  `gorm:"column:video_success_samples;not null;default:0"`
}

func (gpuExecutionCalibrationVideoForM20261118) TableName() string {
	return "gpu_execution_calibrations"
}

type nodeIncentiveVideoForM20261118 struct {
	VideoTaskCount int64 `gorm:"column:video_task_count;not null;default:0"`
}

func (nodeIncentiveVideoForM20261118) TableName() string {
	return "node_incentives"
}

var videoCalibrationColumnsM20261118 = []string{
	"SecondsPerVideoUnit",
	"VideoOverheadSeconds",
	"VideoFormulaVersion",
	"VideoXTX00",
	"VideoXTX01",
	"VideoXTX11",
	"VideoXTY0",
	"VideoXTY1",
	"VideoSuccessSamples",
}

// M20261118 adds video workload units on inference_tasks, video execution calibration columns, and video_task_count on node_incentives.
func M20261118(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261118",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().AddColumn(&inferenceTaskVideoUnitsForM20261118{}, "VideoUnits"); err != nil {
				return err
			}
			for _, col := range videoCalibrationColumnsM20261118 {
				if err := tx.Migrator().AddColumn(&gpuExecutionCalibrationVideoForM20261118{}, col); err != nil {
					return err
				}
			}
			return tx.Migrator().AddColumn(&nodeIncentiveVideoForM20261118{}, "VideoTaskCount")
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropColumn(&nodeIncentiveVideoForM20261118{}, "VideoTaskCount"); err != nil {
				return err
			}
			for i := len(videoCalibrationColumnsM20261118) - 1; i >= 0; i-- {
				if err := tx.Migrator().DropColumn(&gpuExecutionCalibrationVideoForM20261118{}, videoCalibrationColumnsM20261118[i]); err != nil {
					return err
				}
			}
			return tx.Migrator().DropColumn(&inferenceTaskVideoUnitsForM20261118{}, "VideoUnits")
		},
	}})
}
