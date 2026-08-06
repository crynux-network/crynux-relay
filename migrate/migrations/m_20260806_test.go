package migrations

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type inferenceTaskTableForM20260806 struct {
	ID uint `gorm:"primaryKey"`
}

func (inferenceTaskTableForM20260806) TableName() string {
	return "inference_tasks"
}

func TestM20260806AddsLLMMaxNewTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrator().CreateTable(&inferenceTaskTableForM20260806{}); err != nil {
		t.Fatalf("create inference_tasks: %v", err)
	}
	migration := M20260806(db)
	if err := migration.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasColumn(&inferenceTaskLLMMaxNewTokensForM20260806{}, "LLMMaxNewTokens") {
		t.Fatal("expected inference_tasks.LLMMaxNewTokens")
	}
	if err := migration.RollbackLast(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if db.Migrator().HasColumn(&inferenceTaskLLMMaxNewTokensForM20260806{}, "LLMMaxNewTokens") {
		t.Fatal("expected LLMMaxNewTokens removed")
	}
}
