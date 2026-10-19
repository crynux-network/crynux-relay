package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskRequireLinuxCudaForM20261016 struct {
	RequireLinuxCuda bool `gorm:"column:require_linux_cuda;not null;default:false"`
}

func (inferenceTaskRequireLinuxCudaForM20261016) TableName() string { return "inference_tasks" }

type modelRuntimeConstraintForM20261016 struct {
	ID               uint   `gorm:"primaryKey"`
	Repository       string `gorm:"size:512;not null;uniqueIndex:idx_model_runtime_constraints_repo_rev,priority:1"`
	Revision         string `gorm:"size:191;not null;default:'';uniqueIndex:idx_model_runtime_constraints_repo_rev,priority:2"`
	RequireLinuxCuda bool   `gorm:"column:require_linux_cuda;not null;default:false"`
	Source           string `gorm:"size:64;not null;default:''"`
	DetectedAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (modelRuntimeConstraintForM20261016) TableName() string { return "model_runtime_constraints" }

// M20261016 adds RequireLinuxCuda on inference tasks and the Hub detection
// cache table used at task creation.
func M20261016(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{{
		ID: "M20261016",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().AddColumn(&inferenceTaskRequireLinuxCudaForM20261016{}, "RequireLinuxCuda"); err != nil {
				return err
			}
			return tx.Migrator().CreateTable(&modelRuntimeConstraintForM20261016{})
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable(&modelRuntimeConstraintForM20261016{}); err != nil {
				return err
			}
			return tx.Migrator().DropColumn(&inferenceTaskRequireLinuxCudaForM20261016{}, "RequireLinuxCuda")
		},
	}})
}
