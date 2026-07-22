package models

import (
	"context"
	"database/sql"
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
			DoUpdates: clause.Assignments(map[string]interface{}{"total_tasks": gorm.Expr("COALESCE(total_tasks, 0) + 1")}),
		},
	).Model(&taskNumber).Create(&taskNumber).Error; err != nil {
		return err
	}
	return nil
}

type NetworkNodeData struct {
	ID              uint `gorm:"primarykey"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Address         string       `json:"address" gorm:"index"`
	Network         string       `json:"network" gorm:"type:string;size:64;not null;default:'';index"`
	CardModel       string       `json:"card_model"`
	VRam            int          `json:"v_ram"`
	QoS             float64      `json:"qos"`
	Staking         BigInt       `json:"staking" gorm:"type:string;size:255"`
	HealthBase      float64      `json:"health_base"`
	HealthUpdatedAt sql.NullTime `json:"health_updated_at"`
}
