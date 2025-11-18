package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type NodeStaking struct {
	gorm.Model
	NodeAddress      string    `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
	OperatorStaking  BigInt    `json:"operator_staking" gorm:"type:string;size:191;not null"`
	DelegatorStaking BigInt    `json:"delegator_staking" gorm:"type:string;size:191;not null"`
	Time             time.Time `json:"time" gorm:"uniqueIndex:idx_node_time"`
}

func GetNodeStakings(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) ([]NodeStaking, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodeStakings []NodeStaking
	if err := db.WithContext(dbCtx).Model(&NodeStaking{}).Where("node_address = ?", nodeAddress).Where("time >= ?", start).Where("time < ?", end).Order("time desc").Find(&nodeStakings).Error; err != nil {
		return nil, err
	}

	return nodeStakings, nil
}
