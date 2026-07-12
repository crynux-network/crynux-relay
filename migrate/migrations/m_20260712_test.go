package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskTableForM20260712 struct {
	ID uint `gorm:"primarykey"`
}

func (inferenceTaskTableForM20260712) TableName() string {
	return "inference_tasks"
}

type nodeTableForM20260712 struct {
	ID uint `gorm:"primarykey"`
}

func (nodeTableForM20260712) TableName() string {
	return "nodes"
}

func TestM20260712AddsDeliveredTimeAndLastSeenTime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskTableForM20260712{}); err != nil {
		t.Fatalf("failed to create inference_tasks table: %v", err)
	}
	if err := db.Migrator().CreateTable(&nodeTableForM20260712{}); err != nil {
		t.Fatalf("failed to create nodes table: %v", err)
	}

	migration := M20260712(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&inferenceTaskDeliveredTimeMigration{}, "DeliveredTime") {
		t.Fatalf("expected inference_tasks.delivered_time column to exist")
	}
	if !db.Migrator().HasColumn(&nodeLastSeenTimeMigration{}, "LastSeenTime") {
		t.Fatalf("expected nodes.last_seen_time column to exist")
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&inferenceTaskDeliveredTimeMigration{}, "DeliveredTime") {
		t.Fatalf("expected inference_tasks.delivered_time column to be dropped")
	}
	if db.Migrator().HasColumn(&nodeLastSeenTimeMigration{}, "LastSeenTime") {
		t.Fatalf("expected nodes.last_seen_time column to be dropped")
	}
}
