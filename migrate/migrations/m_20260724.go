package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type nodeTaskErrorForM20260724 struct {
	ID               uint      `gorm:"primarykey;index:idx_node_task_errors_created_id,priority:2"`
	CreatedAt        time.Time `gorm:"index:idx_node_task_errors_created_id,priority:1"`
	UpdatedAt        time.Time
	NodeAddress      string `gorm:"type:string;size:42;not null;index;uniqueIndex:idx_node_task_errors_node_task,priority:1"`
	TaskIDCommitment string `gorm:"type:string;size:191;not null;index;uniqueIndex:idx_node_task_errors_node_task,priority:2"`
	TaskArgs         string `gorm:"type:longtext;not null"`
	ErrorType        string `gorm:"type:string;size:64;not null"`
	Message          string `gorm:"type:longtext;not null"`
	StackTrace       string `gorm:"type:longtext;not null"`
	CapturedAt       int64  `gorm:"not null"`
}

func (nodeTaskErrorForM20260724) TableName() string {
	return "node_task_errors"
}

func M20260724(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260724",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().CreateTable(&nodeTaskErrorForM20260724{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(&nodeTaskErrorForM20260724{})
			},
		},
	})
}
