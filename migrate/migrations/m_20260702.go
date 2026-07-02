package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type vestingDelegationEmissionDetailMigration struct {
	ID               uint `gorm:"primaryKey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	VestingRecordID  uint      `gorm:"not null"`
	UserAddress      string    `gorm:"not null;size:64"`
	NodeAddress      string    `gorm:"not null;size:64"`
	Network          string    `gorm:"not null;size:64"`
	TaskFee          string    `gorm:"not null;type:string;size:191"`
	EmissionAmount   string    `gorm:"not null;type:string;size:191"`
	Source           string    `gorm:"not null;size:64"`
	DetailExternalID string    `gorm:"not null;size:191"`
	StartTime        time.Time `gorm:"not null"`
}

func (vestingDelegationEmissionDetailMigration) TableName() string {
	return "vesting_delegation_emission_details"
}

type vestingDelegationEmissionDetailIndexMigration struct {
	VestingRecordID  uint      `gorm:"index:idx_vded_vesting_record_id"`
	UserAddress      string    `gorm:"size:64;index:idx_vded_user_node_network_start,priority:1"`
	NodeAddress      string    `gorm:"size:64;index:idx_vded_node_network_start,priority:1;index:idx_vded_user_node_network_start,priority:2"`
	Network          string    `gorm:"size:64;index:idx_vded_node_network_start,priority:2;index:idx_vded_user_node_network_start,priority:3"`
	Source           string    `gorm:"size:64;uniqueIndex:idx_vded_source_detail_external_id,priority:1"`
	DetailExternalID string    `gorm:"size:191;uniqueIndex:idx_vded_source_detail_external_id,priority:2"`
	StartTime        time.Time `gorm:"index:idx_vded_node_network_start,priority:3;index:idx_vded_user_node_network_start,priority:4"`
}

func (vestingDelegationEmissionDetailIndexMigration) TableName() string {
	return "vesting_delegation_emission_details"
}

func M20260702(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260702",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.CreateTable(&vestingDelegationEmissionDetailMigration{}); err != nil {
					return err
				}
				if err := m.CreateIndex(&vestingDelegationEmissionDetailIndexMigration{}, "idx_vded_vesting_record_id"); err != nil {
					return err
				}
				if err := m.CreateIndex(&vestingDelegationEmissionDetailIndexMigration{}, "idx_vded_node_network_start"); err != nil {
					return err
				}
				if err := m.CreateIndex(&vestingDelegationEmissionDetailIndexMigration{}, "idx_vded_user_node_network_start"); err != nil {
					return err
				}
				return m.CreateIndex(&vestingDelegationEmissionDetailIndexMigration{}, "idx_vded_source_detail_external_id")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&vestingDelegationEmissionDetailMigration{})
			},
		},
	})
}
