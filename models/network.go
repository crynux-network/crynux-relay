package models

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NetworkNodeNumber struct {
	gorm.Model
	AllNodes    uint64 `json:"all_nodes"`
	BusyNodes   uint64 `json:"busy_nodes"`
	ActiveNodes uint64 `json:"active_nodes"`
}

type NetworkTaskNumber struct {
	gorm.Model
	TotalTasks   uint64 `json:"total_tasks"`
	RunningTasks uint64 `json:"running_tasks"`
	QueuedTasks  uint64 `json:"queued_tasks"`
}

func AddTotalTask(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	taskNumber := NetworkTaskNumber{
		Model: gorm.Model{ID: 1},
	}
	if err := db.WithContext(dbCtx).Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"total_tasks": gorm.Expr("total_tasks + 1")}),
		},
	).Model(&taskNumber).Create(&taskNumber).Error; err != nil {
		return err
	}
	return nil
}

type NetworkNodeData struct {
	gorm.Model
	Address   string  `json:"address" gorm:"index"`
	CardModel string  `json:"card_model"`
	VRam      int     `json:"v_ram"`
	Balance   BigInt  `json:"balance" gorm:"type:string;size:255"`
	QoS       float64 `json:"qos"`
	Staking   BigInt  `json:"staking" gorm:"type:string;size:255"`
}
