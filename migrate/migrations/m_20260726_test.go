package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeTaskErrorTableForM20260726 struct {
	ID               uint `gorm:"primarykey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	NodeAddress      string `gorm:"type:string;size:42;not null"`
	TaskIDCommitment string `gorm:"type:string;size:191;not null"`
	TaskArgs         string `gorm:"type:longtext;not null"`
	ErrorType        string `gorm:"type:string;size:64;not null"`
	Message          string `gorm:"type:longtext;not null"`
	StackTrace       string `gorm:"type:longtext;not null"`
	CapturedAt       int64  `gorm:"not null"`
}

func (nodeTaskErrorTableForM20260726) TableName() string {
	return "node_task_errors"
}

func TestM20260726AddsWorkerGpuFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&nodeTaskErrorTableForM20260726{}); err != nil {
		t.Fatalf("failed to create node_task_errors table: %v", err)
	}

	migration := M20260726(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	for _, column := range []string{"GpuCount", "GpuModel", "GpuVramMb", "ExecutorMode"} {
		if !db.Migrator().HasColumn(&nodeTaskErrorGpuFieldsForM20260726{}, column) {
			t.Fatalf("expected node_task_errors.%s column to exist", column)
		}
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	for _, column := range []string{"GpuCount", "GpuModel", "GpuVramMb", "ExecutorMode"} {
		if db.Migrator().HasColumn(&nodeTaskErrorGpuFieldsForM20260726{}, column) {
			t.Fatalf("expected node_task_errors.%s column to be dropped", column)
		}
	}
}
