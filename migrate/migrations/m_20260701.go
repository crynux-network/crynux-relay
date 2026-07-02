package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotAPRMigration struct {
	DelegationApr12m       float64   `gorm:"column:delegation_apr_12m;not null;default:0;index"`
	AprObservationDays     uint32    `gorm:"not null;default:0"`
	DelegationAprUpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (delegatedStakingNodeListSnapshotAPRMigration) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func M20260701(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260701",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, "DelegationApr12m"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, "AprObservationDays"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, "DelegationAprUpdatedAt"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedStakingNodeListSnapshotAPRMigration{}, "DelegationApr12m"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotAPRMigration{}, "DelegationApr12m"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, "DelegationAprUpdatedAt"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, "AprObservationDays"); err != nil {
					return err
				}
				return m.DropColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, "DelegationApr12m")
			},
		},
	})
}
