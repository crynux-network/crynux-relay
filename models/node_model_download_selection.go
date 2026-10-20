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
// index on (model_file_hash, node_address, active) allows many expired history
// rows but at most one non-expired record per required model file set and node.
// ModelFileHash is the DistributionKey of the required model file set that the
// two member lists describe.
type NodeModelDownloadSelection struct {
	ID                            uint                             `json:"id" gorm:"primaryKey"`
	CreatedAt                     time.Time                        `json:"created_at"`
	UpdatedAt                     time.Time                        `json:"updated_at"`
	ModelFileHash                 []byte                           `json:"model_file_hash" gorm:"type:binary(32);not null;uniqueIndex:idx_node_model_download_selection_active,priority:1"`
	RequiredModelFileHashes       []byte                           `json:"-" gorm:"type:longblob;not null"`
	RequiredModelFileSourceHashes []byte                           `json:"-" gorm:"type:longblob;not null"`
	NodeAddress                   string                           `json:"node_address" gorm:"size:42;index;uniqueIndex:idx_node_model_download_selection_active,priority:2"`
	MinVRAM                       uint64                           `json:"min_vram" gorm:"column:min_vram;not null;default:0"`
	SentAt                        time.Time                        `json:"sent_at"`
	Deadline                      time.Time                        `json:"deadline"`
	Status                        NodeModelDownloadSelectionStatus `json:"status" gorm:"size:16;index"`
	Active                        *bool                            `json:"-" gorm:"uniqueIndex:idx_node_model_download_selection_active,priority:3"`
}

// NewNodeModelDownloadSelection builds a pending selection of the required
// model file set made of the exact and source hashes.
func NewNodeModelDownloadSelection(exact, source []Hash32, nodeAddress string, minVRAM uint64, sentAt, deadline time.Time) *NodeModelDownloadSelection {
	active := true
	return &NodeModelDownloadSelection{
		ModelFileHash:                 DistributionKey(exact, source).Bytes(),
		RequiredModelFileHashes:       EncodeHashList(exact),
		RequiredModelFileSourceHashes: EncodeHashList(source),
		NodeAddress:                   nodeAddress,
		MinVRAM:                       minVRAM,
		SentAt:                        sentAt,
		Deadline:                      deadline,
		Status:                        NodeModelDownloadSelectionPending,
		Active:                        &active,
	}
}

// RequiredHashes decodes the exact and source members of the selection.
func (s *NodeModelDownloadSelection) RequiredHashes() (exact, source []Hash32, err error) {
	if exact, err = DecodeHashList(s.RequiredModelFileHashes); err != nil {
		return nil, nil, err
	}
	if source, err = DecodeHashList(s.RequiredModelFileSourceHashes); err != nil {
		return nil, nil, err
	}
	return exact, source, nil
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

func hashesToBytes(hashes []Hash32) [][]byte {
	keys := make([][]byte, 0, len(hashes))
	for _, h := range hashes {
		keys = append(keys, h.Bytes())
	}
	return keys
}

func DeleteNodeModelDownloadSelectionsByModelFileHashes(ctx context.Context, db *gorm.DB, modelFileHashes []Hash32) error {
	if len(modelFileHashes) == 0 {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).
		Where("model_file_hash IN ?", hashesToBytes(modelFileHashes)).
		Delete(&NodeModelDownloadSelection{}).Error
}

func DeleteNodeModelDownloadSelectionsByNodeAddress(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).
		Where("node_address = ?", nodeAddress).
		Delete(&NodeModelDownloadSelection{}).Error
}
