package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type NodeScore struct {
	gorm.Model
	NodeAddress  string    `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
	Time         time.Time `json:"time" gorm:"uniqueIndex:idx_node_time"`
	ProbWeight   float64   `json:"prob_weight"`
	StakingScore float64   `json:"staking_score"`
	QOSScore     float64   `json:"qos_score"`
}

func GetNodeScores(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) ([]NodeScore, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodeScores []NodeScore
	if err := db.WithContext(dbCtx).Model(&NodeScore{}).Where("node_address = ?", nodeAddress).Where("time >= ?", start).Where("time < ?", end).Order("time desc").Find(&nodeScores).Error; err != nil {
		return nil, err
	}

	return nodeScores, nil
}
