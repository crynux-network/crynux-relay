package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type inferenceTaskLLMMaxNewTokensForM20260806 struct {
	LLMMaxNewTokens *uint64 `gorm:"type:bigint unsigned;null;default:null"`
}

func (inferenceTaskLLMMaxNewTokensForM20260806) TableName() string {
	return "inference_tasks"
}

func M20260806(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260806",
			Migrate: func(tx *gorm.DB) error {
				return tx.Migrator().AddColumn(&inferenceTaskLLMMaxNewTokensForM20260806{}, "LLMMaxNewTokens")
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropColumn(&inferenceTaskLLMMaxNewTokensForM20260806{}, "LLMMaxNewTokens")
			},
		},
	})
}
