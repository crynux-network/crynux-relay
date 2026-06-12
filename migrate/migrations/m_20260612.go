package migrations

import (
	"database/sql"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type delegatedSlashJobNonUniqueIndex struct {
	gorm.Model
	NodeAddress              string        `gorm:"not null;size:191;index:idx_delegated_slash_job_node_network"`
	Network                  string        `gorm:"not null;size:191;index:idx_delegated_slash_job_node_network;index;uniqueIndex:idx_delegated_slash_job_node_slash_event"`
	Status                   string        `gorm:"not null;size:32;index"`
	LatestBatchTransactionID sql.NullInt64 `gorm:"index"`
	LastError                sql.NullString
	NodeSlashTxHash          sql.NullString `gorm:"size:191;uniqueIndex:idx_delegated_slash_job_node_slash_event"`
	NodeSlashLogIndex        sql.NullInt64  `gorm:"uniqueIndex:idx_delegated_slash_job_node_slash_event"`
}

func (delegatedSlashJobNonUniqueIndex) TableName() string {
	return "delegated_slash_jobs"
}

type delegatedSlashJobUniqueIndex struct {
	gorm.Model
	NodeAddress              string        `gorm:"not null;size:191;uniqueIndex:idx_delegated_slash_job_node_network"`
	Network                  string        `gorm:"not null;size:191;uniqueIndex:idx_delegated_slash_job_node_network;index"`
	Status                   string        `gorm:"not null;size:32;index"`
	LatestBatchTransactionID sql.NullInt64 `gorm:"index"`
	LastError                sql.NullString
	NodeSlashTxHash          sql.NullString `gorm:"size:191"`
	NodeSlashLogIndex        sql.NullInt64
}

func (delegatedSlashJobUniqueIndex) TableName() string {
	return "delegated_slash_jobs"
}

func M20260612(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260612",
			Migrate: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropIndex(&delegatedSlashJobNonUniqueIndex{}, "idx_delegated_slash_job_node_network"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedSlashJobNonUniqueIndex{}, "idx_delegated_slash_job_node_network"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedSlashJobNonUniqueIndex{}, "NodeSlashTxHash"); err != nil {
					return err
				}
				if err := m.AddColumn(&delegatedSlashJobNonUniqueIndex{}, "NodeSlashLogIndex"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedSlashJobNonUniqueIndex{}, "idx_delegated_slash_job_node_slash_event"); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				m := tx.Migrator()
				if err := m.DropIndex(&delegatedSlashJobNonUniqueIndex{}, "idx_delegated_slash_job_node_slash_event"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedSlashJobNonUniqueIndex{}, "NodeSlashLogIndex"); err != nil {
					return err
				}
				if err := m.DropColumn(&delegatedSlashJobNonUniqueIndex{}, "NodeSlashTxHash"); err != nil {
					return err
				}
				if err := m.DropIndex(&delegatedSlashJobUniqueIndex{}, "idx_delegated_slash_job_node_network"); err != nil {
					return err
				}
				if err := m.CreateIndex(&delegatedSlashJobUniqueIndex{}, "idx_delegated_slash_job_node_network"); err != nil {
					return err
				}
				return nil
			},
		},
	})
}
