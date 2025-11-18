package migrations

import (
	"database/sql"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func M20251110(db *gorm.DB) *gormigrate.Gormigrate {
	type UserStakingEarning struct {
		gorm.Model
		UserAddress string       `json:"user_address" gorm:"uniqueIndex:idx_user_node_network_time;type:string;size:191;not null"`
		NodeAddress string       `json:"node_address" gorm:"uniqueIndex:idx_user_node_network_time;type:string;size:191;not null"`
		Network     string       `json:"network" gorm:"uniqueIndex:idx_user_node_network_time;type:string;size:191;not null"`
		Earning     string       `json:"earning" gorm:"type:string;size:191;not null"`
		Time        sql.NullTime `json:"time" gorm:"uniqueIndex:idx_user_node_network_time"`
	}

	type NodeEarning struct {
		gorm.Model
		NodeAddress      string       `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
		OperatorEarning  string       `json:"operator_earning" gorm:"type:string;size:191;not null"`
		DelegatorEarning string       `json:"delegator_earning" gorm:"type:string;size:191;not null"`
		Time             sql.NullTime `json:"time" gorm:"uniqueIndex:idx_node_time"`
	}

	type UserEarning struct {
		gorm.Model
		UserAddress string       `json:"node_address" gorm:"uniqueIndex:idx_user_time;type:string;size:191;not null"`
		Earning     string       `json:"earning" gorm:"type:string;size:191;not null"`
		Time        sql.NullTime `json:"time" gorm:"uniqueIndex:idx_user_time"`
	}


	type NodeDelegatorCount struct {
		gorm.Model
		NodeAddress string    `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
		Count       uint64    `json:"count"`
		Time        time.Time `json:"time" gorm:"uniqueIndex:idx_node_time"`
	}

	type NodeScore struct {
		gorm.Model
		NodeAddress  string    `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
		Time         time.Time `json:"time" gorm:"uniqueIndex:idx_node_time"`
		ProbWeight   float64   `json:"prob_weight"`
		StakingScore float64   `json:"staking_score"`
		QOSScore     float64   `json:"qos_score"`
	}

	type NodeStaking struct {
		gorm.Model
		NodeAddress      string    `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
		OperatorStaking  string    `json:"operator_staking" gorm:"type:string;size:191;not null"`
		DelegatorStaking string    `json:"delegator_staking" gorm:"type:string;size:191;not null"`
		Time             time.Time `json:"time" gorm:"uniqueIndex:idx_node_time"`
	}

	return gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "M20251110",
			Migrate: func(tx *gorm.DB) error {
				if err := tx.Migrator().CreateTable(&UserStakingEarning{}); err != nil {
					return err
				}
				if err := tx.Migrator().CreateTable(&NodeEarning{}); err != nil {
					return err
				}
				if err := tx.Migrator().CreateTable(&UserEarning{}); err != nil {
					return err
				}
				if err := tx.Migrator().CreateTable(&NodeDelegatorCount{}); err != nil {
					return err
				}
				if err := tx.Migrator().CreateTable(&NodeScore{}); err != nil {
					return err
				}
				if err := tx.Migrator().CreateTable(&NodeStaking{}); err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable(&UserStakingEarning{}) {
					if err := tx.Migrator().DropTable(&UserStakingEarning{}); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable(&NodeEarning{}) {
					if err := tx.Migrator().DropTable(&NodeEarning{}); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable(&UserEarning{}) {
					if err := tx.Migrator().DropTable(&UserEarning{}); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable(&NodeDelegatorCount{}) {
					if err := tx.Migrator().DropTable(&NodeDelegatorCount{}); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable(&NodeScore{}) {
					if err := tx.Migrator().DropTable(&NodeScore{}); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable(&NodeStaking{}) {
					if err := tx.Migrator().DropTable(&NodeStaking{}); err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
}
