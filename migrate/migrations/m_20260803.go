package migrations

import (
	"database/sql"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type withdrawAuthorizationFieldsForM20260803 struct {
	Timestamp sql.NullInt64  `gorm:"null"`
	Signature sql.NullString `gorm:"type:varchar(255);null"`
}

func (withdrawAuthorizationFieldsForM20260803) TableName() string {
	return "withdraw_records"
}

func M20260803(db *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20260803",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().AddColumn(&withdrawAuthorizationFieldsForM20260803{}, "Timestamp"); err != nil {
					return err
				}
				return tx.Migrator().AddColumn(&withdrawAuthorizationFieldsForM20260803{}, "Signature")
			},
			Rollback: func(tx *gorm.DB) error {
				if err := tx.Migrator().DropColumn(&withdrawAuthorizationFieldsForM20260803{}, "Signature"); err != nil {
					return err
				}
				return tx.Migrator().DropColumn(&withdrawAuthorizationFieldsForM20260803{}, "Timestamp")
			},
		},
	})
}
