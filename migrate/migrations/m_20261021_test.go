package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestM20261021CreatesModelWhitelistsTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	migration := M20261021(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	migrator := db.Migrator()
	if !migrator.HasTable(&modelWhitelistForM20261021{}) {
		t.Fatal("expected model_whitelists table")
	}

	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if migrator.HasTable(&modelWhitelistForM20261021{}) {
		t.Fatal("expected model_whitelists table to be dropped")
	}
}
