package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedStakingNodeListSnapshotMigration struct {
	ID                 uint `gorm:"primarykey"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	NodeAddress        string  `gorm:"type:string;size:191;not null;uniqueIndex:idx_delegated_staking_node_snapshots_address"`
	Network            string  `gorm:"type:string;size:64;not null;index"`
	Status             uint8   `gorm:"not null;index"`
	StatusGroup        string  `gorm:"type:string;size:32;not null;index"`
	StatusRank         uint8   `gorm:"not null;index"`
	GPUName            string  `gorm:"type:string;size:191;not null;index"`
	GPUVram            uint64  `gorm:"not null;index"`
	Version            string  `gorm:"type:string;size:64;not null;index"`
	OperatorEmission4w string  `gorm:"column:operator_emission_4w;type:decimal(65,0);not null;default:0;index"`
	OperatorStaking    string  `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegatorStaking   string  `gorm:"type:decimal(65,0);not null;default:0;index"`
	TotalStaking       string  `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegatorsNum      uint64  `gorm:"not null;default:0;index"`
	ProbWeight         float64 `gorm:"not null;default:0;index"`
	QOS                float64 `gorm:"column:qos;not null;default:0;index"`
}

func (delegatedStakingNodeListSnapshotMigration) TableName() string {
	return "delegated_staking_node_list_snapshots"
}

func M20260627(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260627",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().CreateTable(&delegatedStakingNodeListSnapshotMigration{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&delegatedStakingNodeListSnapshotMigration{})
			},
		},
	})
}
