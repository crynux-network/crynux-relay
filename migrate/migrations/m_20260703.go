package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotEmissionEstimateMigration struct {
	EstimatedUpcomingOperatorEmission  string `gorm:"type:decimal(65,0);not null;default:0;index"`
	EstimatedUpcomingDelegatorEmission string `gorm:"type:decimal(65,0);not null;default:0;index"`
}

func (delegatedStakingNodeListSnapshotEmissionEstimateMigration) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func M20260703(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260703",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingOperatorEmission"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingOperatorEmission"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingDelegatorEmission"); err != nil {
					return err
				}
				return m.CreateIndex(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingDelegatorEmission")
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingDelegatorEmission"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingDelegatorEmission"); err != nil {
					return err
				}
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingOperatorEmission"); err != nil {
					return err
				}
				return m.DropColumn(&delegatedStakingNodeListSnapshotEmissionEstimateMigration{}, "EstimatedUpcomingOperatorEmission")
			},
		},
	})
}
