package migrations

import (
	"database/sql"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20260601(db *gorm.DB) *gormigrate.Gormigrate {
	type DelegatedSlashJob struct {
		gorm.Model
		NodeAddress              string        `gorm:"not null;size:191;uniqueIndex:idx_delegated_slash_job_node_network"`
		Network                  string        `gorm:"not null;size:191;uniqueIndex:idx_delegated_slash_job_node_network;index"`
		Status                   string        `gorm:"not null;size:32;index"`
		LatestBatchTransactionID sql.NullInt64 `gorm:"index"`
		LastError                sql.NullString
	}

	type DelegatedStakingSlashRecord struct {
		gorm.Model
		SlashJobID       sql.NullInt64 `gorm:"index"`
		NodeAddress      string        `gorm:"not null;size:191;index:idx_delegated_staking_slash_record_node_delegator"`
		DelegatorAddress string        `gorm:"not null;size:191;index:idx_delegated_staking_slash_record_node_delegator"`
		Network          string        `gorm:"not null;size:191;index;uniqueIndex:idx_delegated_staking_slash_record_event;index:idx_delegated_staking_slash_record_node_delegator"`
		Amount           string        `gorm:"type:string;size:191;not null"`
		SlashTxHash      string        `gorm:"not null;size:191;uniqueIndex:idx_delegated_staking_slash_record_event"`
		BlockNumber      uint64        `gorm:"not null;index"`
		LogIndex         uint          `gorm:"not null;uniqueIndex:idx_delegated_staking_slash_record_event"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260601",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.CreateTable(&DelegatedSlashJob{}); err != nil {
					return err
				}
				if err := m.CreateTable(&DelegatedStakingSlashRecord{}); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropTable("delegated_staking_slash_records"); err != nil {
					return err
				}
				if err := m.DropTable("delegated_slash_jobs"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
