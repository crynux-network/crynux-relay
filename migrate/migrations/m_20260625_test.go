package migrations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type networkNodeDataRemoveDeletedAtMigrationTestBefore struct {
	gorm.Model
	Address string `gorm:"index"`
	Balance string `gorm:"type:string;size:255"`
}

func (networkNodeDataRemoveDeletedAtMigrationTestBefore) TableName() string {
	return "network_node_data"
}

type networkNodeDataRemoveDeletedAtMigrationTestAfter struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Address   string `gorm:"index"`
}

func (networkNodeDataRemoveDeletedAtMigrationTestAfter) TableName() string {
	return "network_node_data"
}

func TestM20260625RestoresNetworkNodeDataRowsAndDropsDeletedAt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&networkNodeDataRemoveDeletedAtMigrationTestBefore{}); err != nil {
		t.Fatalf("failed to migrate test table: %v", err)
	}

	visible := networkNodeDataRemoveDeletedAtMigrationTestBefore{Address: "0xvisible", Balance: "100"}
	deleted := networkNodeDataRemoveDeletedAtMigrationTestBefore{Address: "0xdeleted", Balance: "200"}
	for _, row := range []*networkNodeDataRemoveDeletedAtMigrationTestBefore{&visible, &deleted} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("failed to seed network node data row: %v", err)
		}
	}
	if err := db.Delete(&deleted).Error; err != nil {
		t.Fatalf("failed to soft-delete network node data row: %v", err)
	}

	if err := M20260625(db).Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var rows []networkNodeDataRemoveDeletedAtMigrationTestAfter
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("failed to load network node data rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("unexpected row count after migration: got %d, want %d", len(rows), 2)
	}
	if rows[0].Address != visible.Address {
		t.Fatalf("expected visible row to remain queryable, got %q", rows[0].Address)
	}
	if rows[1].Address != deleted.Address {
		t.Fatalf("expected restored row to be queryable, got %q", rows[1].Address)
	}
	if db.Migrator().HasColumn(&networkNodeDataRemoveDeletedAtMigrationTestBefore{}, "DeletedAt") {
		t.Fatalf("expected deleted_at column to be dropped")
	}
	if db.Migrator().HasColumn(&networkNodeDataRemoveDeletedAtMigrationTestBefore{}, "Balance") {
		t.Fatalf("expected balance column to be dropped")
	}
}
