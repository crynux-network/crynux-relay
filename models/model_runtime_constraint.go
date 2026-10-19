package models

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ModelRuntimeConstraint caches Hub-derived Linux-CUDA-only detection for one
// repository revision.
type ModelRuntimeConstraint struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	Repository       string    `json:"repository" gorm:"size:512;not null;uniqueIndex:idx_model_runtime_constraints_repo_rev,priority:1"`
	Revision         string    `json:"revision" gorm:"size:191;not null;default:'';uniqueIndex:idx_model_runtime_constraints_repo_rev,priority:2"`
	RequireLinuxCuda bool      `json:"require_linux_cuda" gorm:"column:require_linux_cuda;not null;default:false"`
	Source           string    `json:"source" gorm:"size:64;not null;default:''"`
	DetectedAt       time.Time `json:"detected_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (ModelRuntimeConstraint) TableName() string { return "model_runtime_constraints" }

// GetModelRuntimeConstraint returns the cached constraint for repository and
// revision. ok is false when no row exists.
func GetModelRuntimeConstraint(ctx context.Context, db *gorm.DB, repository, revision string) (*ModelRuntimeConstraint, bool, error) {
	var row ModelRuntimeConstraint
	err := db.WithContext(ctx).
		Where("repository = ? AND revision = ?", repository, revision).
		Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

// UpsertModelRuntimeConstraint writes the Hub detection result for one
// repository revision.
func UpsertModelRuntimeConstraint(ctx context.Context, db *gorm.DB, row *ModelRuntimeConstraint) error {
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "repository"}, {Name: "revision"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"require_linux_cuda", "source", "detected_at", "updated_at",
		}),
	}).Create(row).Error
}
