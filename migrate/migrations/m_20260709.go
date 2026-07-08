package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotEstimatedNextAPRMigration struct {
	EstimatedNext10kDelegationApr  float64 `gorm:"column:estimated_next_10k_delegation_apr;not null;default:0;index"`
	EstimatedNext100kDelegationApr float64 `gorm:"column:estimated_next_100k_delegation_apr;not null;default:0;index"`
	EstimatedNext1mDelegationApr   float64 `gorm:"column:estimated_next_1m_delegation_apr;not null;default:0;index"`
}

func (delegatedStakingNodeListSnapshotEstimatedNextAPRMigration) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func M20260709(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260709",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext10kDelegationApr"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext100kDelegationApr"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext1mDelegationApr"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext10kDelegationApr"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext100kDelegationApr"); err != nil {
					return err
				}
				return m.CreateIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext1mDelegationApr")
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext1mDelegationApr"); err != nil {
					return err
				}
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext100kDelegationApr"); err != nil {
					return err
				}
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext10kDelegationApr"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext1mDelegationApr"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext100kDelegationApr"); err != nil {
					return err
				}
				return m.DropColumn(&delegatedStakingNodeListSnapshotEstimatedNextAPRMigration{}, "EstimatedNext10kDelegationApr")
			},
		},
	})
}
