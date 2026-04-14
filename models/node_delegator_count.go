package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type NodeDelegatorCount struct {
	gorm.Model
	NodeAddress string    `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
	Count       uint64    `json:"count"`
	Time        time.Time `json:"time" gorm:"uniqueIndex:idx_node_time"`
}

func GetNodeDelegatorCounts(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) ([]NodeDelegatorCount, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodeDelegatorCounts []NodeDelegatorCount
	if err := db.WithContext(dbCtx).Model(&NodeDelegatorCount{}).Where("node_address = ?", nodeAddress).Where("time >= ?", start).Where("time < ?", end).Order("time desc").Find(&nodeDelegatorCounts).Error; err != nil {
		return nil, err
	}

	return nodeDelegatorCounts, nil
}