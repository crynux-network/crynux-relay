package models

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NodeInMemoryModel is the model a node currently holds in memory. A node with
// an empty memory has no row.
type NodeInMemoryModel struct {
	NodeAddress             string    `json:"node_address" gorm:"size:191;primaryKey"`
	CurrentModelCacheHash   []byte    `json:"current_model_cache_hash" gorm:"type:binary(32);not null"`
	CurrentModelFileHashes  []byte    `json:"current_model_file_hashes" gorm:"type:longblob;not null"`
	CurrentModelCacheConfig JSONText  `json:"current_model_cache_config" gorm:"type:json;not null"`
	ExecutionDType          string    `json:"execution_dtype" gorm:"column:execution_dtype;size:64;not null"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// NewNodeInMemoryModel builds the in-memory model row of a node. The file
// hashes are stored as a sorted concatenation of 32-byte hashes.
func NewNodeInMemoryModel(nodeAddress string, cacheHash Hash32, fileHashes []Hash32, cacheConfig []byte, executionDType string) NodeInMemoryModel {
	return NodeInMemoryModel{
		NodeAddress:             nodeAddress,
		CurrentModelCacheHash:   cacheHash.Bytes(),
		CurrentModelFileHashes:  EncodeHashList(fileHashes),
		CurrentModelCacheConfig: cacheConfig,
		ExecutionDType:          executionDType,
	}
}

func (m *NodeInMemoryModel) FileHashes() ([]Hash32, error) {
	return DecodeHashList(m.CurrentModelFileHashes)
}

// SaveNodeInMemoryModel replaces the in-memory model of a node.
func SaveNodeInMemoryModel(ctx context.Context, db *gorm.DB, m *NodeInMemoryModel) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_address"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"current_model_cache_hash",
			"current_model_file_hashes",
			"current_model_cache_config",
			"execution_dtype",
			"updated_at",
		}),
	}).Create(m).Error
}

// GetNodeInMemoryModel returns the in-memory model of a node, or nil when the
// node memory is empty.
func GetNodeInMemoryModel(ctx context.Context, db *gorm.DB, nodeAddress string) (*NodeInMemoryModel, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var m NodeInMemoryModel
	err := db.WithContext(dbCtx).Where("node_address = ?", nodeAddress).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ClearNodeInMemoryModel marks the node memory as empty.
func ClearNodeInMemoryModel(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Where("node_address = ?", nodeAddress).Delete(&NodeInMemoryModel{}).Error
}
