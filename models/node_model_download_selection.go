package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type NodeModelDownloadSelectionStatus string

const (
	NodeModelDownloadSelectionPending   NodeModelDownloadSelectionStatus = "pending"
	NodeModelDownloadSelectionCompleted NodeModelDownloadSelectionStatus = "completed"
	NodeModelDownloadSelectionExpired   NodeModelDownloadSelectionStatus = "expired"
)

// NodeModelDownloadSelection is one download selection attempt made by the
// model distribution controller. The Active marker is true while the record
// is non-expired (pending or completed) and NULL once expired, so the unique
// index on (model_id, node_address, active) allows many expired history rows
// but at most one non-expired record per model and node.
type NodeModelDownloadSelection struct {
	ID          uint                             `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time                        `json:"created_at"`
	UpdatedAt   time.Time                        `json:"updated_at"`
	ModelID     string                           `json:"model_id" gorm:"size:255;uniqueIndex:idx_node_model_download_selection_active,priority:1"`
	NodeAddress string                           `json:"node_address" gorm:"size:42;index;uniqueIndex:idx_node_model_download_selection_active,priority:2"`
	MinVRAM     uint64                           `json:"min_vram" gorm:"column:min_vram;not null;default:0"`
	SentAt      time.Time                        `json:"sent_at"`
	Deadline    time.Time                        `json:"deadline"`
	Status      NodeModelDownloadSelectionStatus `json:"status" gorm:"size:16;index"`
	Active      *bool                            `json:"-" gorm:"uniqueIndex:idx_node_model_download_selection_active,priority:3"`
}

func (selection *NodeModelDownloadSelection) BeforeSave(tx *gorm.DB) error {
	selection.ModelID = NormalizeModelID(selection.ModelID)
	return nil
}

func NewNodeModelDownloadSelection(modelID, nodeAddress string, minVRAM uint64, sentAt, deadline time.Time) *NodeModelDownloadSelection {
	active := true
	return &NodeModelDownloadSelection{
		ModelID:     NormalizeModelID(modelID),
		NodeAddress: nodeAddress,
		MinVRAM:     minVRAM,
		SentAt:      sentAt,
		Deadline:    deadline,
		Status:      NodeModelDownloadSelectionPending,
		Active:      &active,
	}
}

func CreateNodeModelDownloadSelection(ctx context.Context, db *gorm.DB, selection *NodeModelDownloadSelection) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Create(selection).Error
}

func GetAllNodeModelDownloadSelections(ctx context.Context, db *gorm.DB) ([]NodeModelDownloadSelection, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var selections []NodeModelDownloadSelection
	if err := db.WithContext(dbCtx).Model(&NodeModelDownloadSelection{}).Order("id").Find(&selections).Error; err != nil {
		return nil, err
	}
	return selections, nil
}

func CompleteNodeModelDownloadSelections(ctx context.Context, db *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Model(&NodeModelDownloadSelection{}).
		Where("id IN ?", ids).
		Where("status = ?", NodeModelDownloadSelectionPending).
		Update("status", NodeModelDownloadSelectionCompleted).Error
}

func ExpireNodeModelDownloadSelections(ctx context.Context, db *gorm.DB, now time.Time) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Model(&NodeModelDownloadSelection{}).
		Where("status = ?", NodeModelDownloadSelectionPending).
		Where("deadline < ?", now).
		Updates(map[string]interface{}{
			"status": NodeModelDownloadSelectionExpired,
			"active": nil,
		}).Error
}

func DeleteNodeModelDownloadSelectionsByModelIDs(ctx context.Context, db *gorm.DB, modelIDs []string) error {
	if len(modelIDs) == 0 {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).
		Where("model_id IN ?", modelIDs).
		Delete(&NodeModelDownloadSelection{}).Error
}

func DeleteNodeModelDownloadSelectionsByNodeAddress(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).
		Where("node_address = ?", nodeAddress).
		Delete(&NodeModelDownloadSelection{}).Error
}
