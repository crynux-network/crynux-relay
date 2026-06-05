package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type BlockchainCursor struct {
	gorm.Model
	Network        string    `json:"network" gorm:"not null"`
	LastBlockNum   uint64    `json:"last_block_num" gorm:"not null;default:0"`
	LastUpdateTime time.Time `json:"last_update_time" gorm:"not null"`
}

func GetBlockchainCursor(ctx context.Context, db *gorm.DB, network string, startBlockNum uint64) (*BlockchainCursor, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cursor BlockchainCursor
	err := db.WithContext(dbCtx).Model(&BlockchainCursor{}).Where("network = ?", network).Attrs(&BlockchainCursor{
		LastBlockNum:   startBlockNum,
		LastUpdateTime: time.Now(),
		Network:        network,
	}).FirstOrCreate(&cursor).Error

	if err != nil {
		return nil, err
	}

	return &cursor, nil
}
