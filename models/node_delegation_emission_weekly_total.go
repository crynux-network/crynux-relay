package models

import (
	"context"
	"math/big"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeDelegationEmissionWeeklyTotal struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	NodeAddress    string    `json:"node_address" gorm:"not null;size:64;uniqueIndex:idx_ndewt_node_start,priority:1;index:idx_ndewt_node_start_range,priority:1"`
	StartTime      time.Time `json:"start_time" gorm:"not null;uniqueIndex:idx_ndewt_node_start,priority:2;index:idx_ndewt_node_start_range,priority:2"`
	EmissionAmount BigInt    `json:"emission_amount" gorm:"not null;type:decimal(65,0);default:0"`
}

type NodeDelegationEmissionWeeklyIncrement struct {
	NodeAddress    string
	StartTime      time.Time
	EmissionAmount *big.Int
}

func ListNodeDelegationEmissionWeeklyTotalsByNodeAndStartTimeRange(ctx context.Context, db *gorm.DB, nodeAddress string, startTime, endTime time.Time) ([]NodeDelegationEmissionWeeklyTotal, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var totals []NodeDelegationEmissionWeeklyTotal
	if err := db.WithContext(dbCtx).
		Model(&NodeDelegationEmissionWeeklyTotal{}).
		Where("node_address = ?", nodeAddress).
		Where("start_time >= ? AND start_time < ?", startTime, endTime).
		Order("start_time ASC").
		Find(&totals).Error; err != nil {
		return nil, err
	}
	return totals, nil
}

func UpsertNodeDelegationEmissionWeeklyTotalIncrements(ctx context.Context, db *gorm.DB, increments []NodeDelegationEmissionWeeklyIncrement) error {
	if len(increments) == 0 {
		return nil
	}

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows := make([]NodeDelegationEmissionWeeklyTotal, 0, len(increments))
	for _, increment := range increments {
		if increment.EmissionAmount == nil || increment.EmissionAmount.Sign() == 0 {
			continue
		}
		rows = append(rows, NodeDelegationEmissionWeeklyTotal{
			NodeAddress:    increment.NodeAddress,
			StartTime:      increment.StartTime.UTC(),
			EmissionAmount: BigInt{Int: *new(big.Int).Set(increment.EmissionAmount)},
		})
	}
	if len(rows) == 0 {
		return nil
	}

	amountExpr := gorm.Expr("emission_amount + VALUES(emission_amount)")
	updatedAtExpr := gorm.Expr("VALUES(updated_at)")
	if db.Dialector.Name() == "sqlite" {
		amountExpr = gorm.Expr("emission_amount + excluded.emission_amount")
		updatedAtExpr = gorm.Expr("excluded.updated_at")
	}

	return db.WithContext(dbCtx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "node_address"},
			{Name: "start_time"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"emission_amount": amountExpr,
			"updated_at":      updatedAtExpr,
		}),
	}).Create(&rows).Error
}
