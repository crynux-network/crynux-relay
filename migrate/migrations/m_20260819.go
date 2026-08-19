package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type gpuExecutionCalibrationSDFitForM20260819 struct {
	SDOverheadSeconds float64 `gorm:"not null;default:0"`
	SDFormulaVersion  uint64  `gorm:"not null;default:0"`
	SDXTX00           float64 `gorm:"column:sd_xtx_00;not null;default:0"`
	SDXTX01           float64 `gorm:"column:sd_xtx_01;not null;default:0"`
	SDXTX11           float64 `gorm:"column:sd_xtx_11;not null;default:0"`
	SDXTY0            float64 `gorm:"column:sd_xty_0;not null;default:0"`
	SDXTY1            float64 `gorm:"column:sd_xty_1;not null;default:0"`
}

func (gpuExecutionCalibrationSDFitForM20260819) TableName() string {
	return "gpu_execution_calibrations"
}

var gpuExecutionCalibrationSDFitColumnsForM20260819 = []string{
	"SDOverheadSeconds", "SDFormulaVersion",
	"SDXTX00", "SDXTX01", "SDXTX11",
	"SDXTY0", "SDXTY1",
}

func M20260819(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20260819",
		Migrate: func(tx *gorm.DB) error {
			for _, column := range gpuExecutionCalibrationSDFitColumnsForM20260819 {
				if err := tx.Migrator().AddColumn(&gpuExecutionCalibrationSDFitForM20260819{}, column); err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			for i := len(gpuExecutionCalibrationSDFitColumnsForM20260819) - 1; i >= 0; i-- {
				if err := tx.Migrator().DropColumn(
					&gpuExecutionCalibrationSDFitForM20260819{},
					gpuExecutionCalibrationSDFitColumnsForM20260819[i],
				); err != nil {
					return err
				}
			}
			return nil
		},
	}})
}
