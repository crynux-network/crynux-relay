package migrations

import (
	"errors"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type vestingRecordBusinessKeyMigration struct {
	Type       string    `gorm:"size:32;uniqueIndex:idx_vesting_type_address_start,priority:1"`
	Address    string    `gorm:"uniqueIndex:idx_vesting_type_address_start,priority:2"`
	StartTime  time.Time `gorm:"uniqueIndex:idx_vesting_type_address_start,priority:3"`
	Source     string    `gorm:"size:64;uniqueIndex:idx_vesting_source_external_id"`
	ExternalID string    `gorm:"size:128;uniqueIndex:idx_vesting_source_external_id"`
}

func (vestingRecordBusinessKeyMigration) TableName() string {
	return "vesting_records"
}

type vestingDelegationEmissionDetailBusinessKeyMigration struct {
	UserAddress      string    `gorm:"size:64;uniqueIndex:idx_vded_user_node_network_start,priority:1"`
	NodeAddress      string    `gorm:"size:64;uniqueIndex:idx_vded_user_node_network_start,priority:2"`
	Network          string    `gorm:"size:64;uniqueIndex:idx_vded_user_node_network_start,priority:3"`
	StartTime        time.Time `gorm:"uniqueIndex:idx_vded_user_node_network_start,priority:4"`
	Source           string    `gorm:"size:64;uniqueIndex:idx_vded_source_detail_external_id,priority:1"`
	DetailExternalID string    `gorm:"size:191;uniqueIndex:idx_vded_source_detail_external_id,priority:2"`
}

func (vestingDelegationEmissionDetailBusinessKeyMigration) TableName() string {
	return "vesting_delegation_emission_details"
}

func hasVestingRecordBusinessKeyDuplicates(tx *gorm.DB) (bool, error) {
	var count int64
	err := tx.Table("vesting_records").
		Select("COUNT(*)").
		Group("type, address, start_time").
		Having("COUNT(*) > 1").
		Limit(1).
		Scan(&count).Error
	return count > 0, err
}

func hasVestingDelegationDetailBusinessKeyDuplicates(tx *gorm.DB) (bool, error) {
	var count int64
	err := tx.Table("vesting_delegation_emission_details").
		Select("COUNT(*)").
		Group("user_address, node_address, network, start_time").
		Having("COUNT(*) > 1").
		Limit(1).
		Scan(&count).Error
	return count > 0, err
}

func M20260705(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260705",
			Migrate: func(tx *gorm.DB) error {
				hasDuplicates, err := hasVestingRecordBusinessKeyDuplicates(tx)
				if err != nil {
					return err
				}
				if hasDuplicates {
					return errors.New("vesting_records contains duplicate type, address, and start_time groups")
				}

				hasDuplicates, err = hasVestingDelegationDetailBusinessKeyDuplicates(tx)
				if err != nil {
					return err
				}
				if hasDuplicates {
					return errors.New("vesting_delegation_emission_details contains duplicate user_address, node_address, network, and start_time groups")
				}

				m := tx.Migrator()
				if err := m.DropIndex(&vestingRecordBusinessKeyMigration{}, "idx_vesting_source_external_id"); err != nil {
					return err
				}
				if err := m.DropColumn(&vestingRecordBusinessKeyMigration{}, "Source"); err != nil {
					return err
				}
				if err := m.DropColumn(&vestingRecordBusinessKeyMigration{}, "ExternalID"); err != nil {
					return err
				}
				if err := m.CreateIndex(&vestingRecordBusinessKeyMigration{}, "idx_vesting_type_address_start"); err != nil {
					return err
				}

				if err := m.DropIndex(&vestingDelegationEmissionDetailBusinessKeyMigration{}, "idx_vded_source_detail_external_id"); err != nil {
					return err
				}
				if err := m.DropColumn(&vestingDelegationEmissionDetailBusinessKeyMigration{}, "Source"); err != nil {
					return err
				}
				if err := m.DropColumn(&vestingDelegationEmissionDetailBusinessKeyMigration{}, "DetailExternalID"); err != nil {
					return err
				}
				if err := m.DropIndex(&vestingDelegationEmissionDetailBusinessKeyMigration{}, "idx_vded_user_node_network_start"); err != nil {
					return err
				}
				return m.CreateIndex(&vestingDelegationEmissionDetailBusinessKeyMigration{}, "idx_vded_user_node_network_start")
			},
			Rollback: func(tx *gorm.DB) error {
				return errors.New("M20260705 rollback is not supported after vesting source and external id columns are removed")
			},
		},
	})
}
