package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotDelegatorEmission4wMigration struct {
	DelegatorEmission4w string `gorm:"column:delegator_emission_4w;type:decimal(65,0);not null;default:0;index"`
}

func (delegatedStakingNodeListSnapshotDelegatorEmission4wMigration) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func M20260704(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260704",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.AddColumn(&delegatedStakingNodeListSnapshotDelegatorEmission4wMigration{}, "DelegatorEmission4w"); err != nil {
					return err
				}
				return m.CreateIndex(&delegatedStakingNodeListSnapshotDelegatorEmission4wMigration{}, "DelegatorEmission4w")
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropIndex(&delegatedStakingNodeListSnapshotDelegatorEmission4wMigration{}, "DelegatorEmission4w"); err != nil {
					return err
				}
				return m.DropColumn(&delegatedStakingNodeListSnapshotDelegatorEmission4wMigration{}, "DelegatorEmission4w")
			},
		},
	})
}
