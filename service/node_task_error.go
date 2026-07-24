package service

import (
	"context"
	"crynux_relay/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeTaskErrorFilter struct {
	NodeAddress      string
	TaskIDCommitment string
}

func CreateNodeTaskError(ctx context.Context, db *gorm.DB, record *models.NodeTaskError) (bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := db.WithContext(dbCtx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "node_address"},
				{Name: "task_id_commitment"},
			},
			DoNothing: true,
		}).
		Create(record)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func ListNodeTaskErrors(
	ctx context.Context,
	db *gorm.DB,
	filter NodeTaskErrorFilter,
	page int,
	pageSize int,
) ([]models.NodeTaskError, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := db.WithContext(dbCtx).Model(&models.NodeTaskError{})
	if filter.NodeAddress != "" {
		query = query.Where("node_address = ?", filter.NodeAddress)
	}
	if filter.TaskIDCommitment != "" {
		query = query.Where("task_id_commitment = ?", filter.TaskIDCommitment)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	records := make([]models.NodeTaskError, 0)
	if err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
