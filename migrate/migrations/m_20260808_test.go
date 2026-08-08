package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nodeTableForM20260808 struct {
	ID uint `gorm:"primaryKey"`
}

func (nodeTableForM20260808) TableName() string {
	return "nodes"
}

func TestM20260808AddsHealthExcluded(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&nodeTableForM20260808{}); err != nil {
		t.Fatalf("create nodes: %v", err)
	}
	migration := M20260808(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasColumn(&nodeHealthExcludedForM20260808{}, "HealthExcluded") {
		t.Fatal("expected nodes.HealthExcluded")
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if db.Migrator().HasColumn(&nodeHealthExcludedForM20260808{}, "HealthExcluded") {
		t.Fatal("expected HealthExcluded removed")
	}
}
