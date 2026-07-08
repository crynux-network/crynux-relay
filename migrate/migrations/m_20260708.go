package migrations

import (
	"math/big"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeDelegationEmissionWeeklyTotalMigration struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	NodeAddress    string    `gorm:"not null;size:64;uniqueIndex:idx_ndewt_node_start,priority:1;index:idx_ndewt_node_start_range,priority:1"`
	StartTime      time.Time `gorm:"not null;uniqueIndex:idx_ndewt_node_start,priority:2;index:idx_ndewt_node_start_range,priority:2"`
	EmissionAmount string    `gorm:"not null;type:decimal(65,0);default:0"`
}

func (nodeDelegationEmissionWeeklyTotalMigration) TableName() string {
	return "node_delegation_emission_weekly_totals"
}

type nodeDelegationEmissionWeeklyBackfillRow struct {
	NodeAddress    string
	StartTime      time.Time
	EmissionAmount string
}

func backfillNodeDelegationEmissionWeeklyTotals(tx *gorm.DB) error {
	var rows []nodeDelegationEmissionWeeklyBackfillRow
	if err := tx.Table("vesting_delegation_emission_details").
		Select("node_address, start_time, SUM(CAST(emission_amount AS DECIMAL(65,0))) AS emission_amount").
		Group("node_address, start_time").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	totals := make([]nodeDelegationEmissionWeeklyTotalMigration, 0, len(rows))
	now := time.Now().UTC()
	for _, row := range rows {
		amount, ok := big.NewInt(0).SetString(row.EmissionAmount, 10)
		if !ok || amount.Sign() == 0 {
			continue
		}
		totals = append(totals, nodeDelegationEmissionWeeklyTotalMigration{
			CreatedAt:      now,
			UpdatedAt:      now,
			NodeAddress:    row.NodeAddress,
			StartTime:      row.StartTime.UTC(),
			EmissionAmount: amount.String(),
		})
	}
	if len(totals) == 0 {
		return nil
	}
	return tx.Create(&totals).Error
}

func M20260708(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260708",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().CreateTable(&nodeDelegationEmissionWeeklyTotalMigration{}); err != nil {
					return err
				}
				return backfillNodeDelegationEmissionWeeklyTotals(tx)
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&nodeDelegationEmissionWeeklyTotalMigration{})
			},
		},
	})
}
