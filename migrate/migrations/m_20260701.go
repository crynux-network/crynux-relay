package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotAPRMigration struct {
	DelegationApr12m       float64 `gorm:"column:delegation_apr_12m;not null;default:0;index"`
	AprObservationDays     uint32  `gorm:"not null;default:0"`
	DelegationAprUpdatedAt string  `gorm:"type:datetime(3);not null;default:'1970-01-01 00:00:00.000'"`
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
				if err := addDelegatedStakingNodeListSnapshotAPRColumn(m, "DelegationApr12m"); err != nil {
					return err
				}
				if err := addDelegatedStakingNodeListSnapshotAPRColumn(m, "AprObservationDays"); err != nil {
					return err
				}
				if err := addDelegatedStakingNodeListSnapshotAPRColumn(m, "DelegationAprUpdatedAt"); err != nil {
					return err
				}
				if err := createDelegatedStakingNodeListSnapshotAPRIndex(m, "DelegationApr12m"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := dropDelegatedStakingNodeListSnapshotAPRIndex(m, "DelegationApr12m"); err != nil {
					return err
				}
				if err := dropDelegatedStakingNodeListSnapshotAPRColumn(m, "DelegationAprUpdatedAt"); err != nil {
					return err
				}
				if err := dropDelegatedStakingNodeListSnapshotAPRColumn(m, "AprObservationDays"); err != nil {
					return err
				}
				return dropDelegatedStakingNodeListSnapshotAPRColumn(m, "DelegationApr12m")
			},
		},
	})
}

func addDelegatedStakingNodeListSnapshotAPRColumn(m gorm.Migrator, name string) error {
	if m.HasColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, name) {
		return nil
	}
	return m.AddColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, name)
}

func createDelegatedStakingNodeListSnapshotAPRIndex(m gorm.Migrator, name string) error {
	if m.HasIndex(&delegatedStakingNodeListSnapshotAPRMigration{}, name) {
		return nil
	}
	return m.CreateIndex(&delegatedStakingNodeListSnapshotAPRMigration{}, name)
}

func dropDelegatedStakingNodeListSnapshotAPRColumn(m gorm.Migrator, name string) error {
	if !m.HasColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, name) {
		return nil
	}
	return m.DropColumn(&delegatedStakingNodeListSnapshotAPRMigration{}, name)
}

func dropDelegatedStakingNodeListSnapshotAPRIndex(m gorm.Migrator, name string) error {
	if !m.HasIndex(&delegatedStakingNodeListSnapshotAPRMigration{}, name) {
		return nil
	}
	return m.DropIndex(&delegatedStakingNodeListSnapshotAPRMigration{}, name)
}
