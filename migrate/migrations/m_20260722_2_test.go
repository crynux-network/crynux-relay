package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type networkNodeDataTableForM20260722_2 struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Address   string `gorm:"index"`
}

func (networkNodeDataTableForM20260722_2) TableName() string {
	return "network_node_data"
}

func TestM20260722_2AddsNetwork(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.Migrator().CreateTable(&networkNodeDataTableForM20260722_2{}); err != nil {
		t.Fatalf("failed to create network_node_data table: %v", err)
	}

	migration := M20260722_2(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&networkNodeDataNetworkForM20260722_2{}, "Network") {
		t.Fatalf("expected network_node_data.network column to exist")
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if db.Migrator().HasColumn(&networkNodeDataNetworkForM20260722_2{}, "Network") {
		t.Fatalf("expected network_node_data.network column to be dropped")
	}
}
