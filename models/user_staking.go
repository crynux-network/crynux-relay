package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type UserStaking struct {
	gorm.Model
	UserAddress string `json:"user_address" gorm:"index;type:string;size:191;not null"`
	NodeAddress string `json:"node_address" gorm:"index;type:string;size:191;not null"`
	Amount      BigInt `json:"amount" gorm:"type:string;size:191;not null"`
	Valid       bool   `json:"valid" gorm:"not null;index"`
	Network     string `json:"network" gorm:"not null;index;type:string;size:191"`
}

func GetUserStakingsOfUser(ctx context.Context, db *gorm.DB, userAddress string, network *string) ([]UserStaking, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStakings []UserStaking
	dbi := db.WithContext(dbCtx).Model(&UserStaking{}).Where("user_address = ?", userAddress).Where("valid = ?", true)
	if network != nil {
		dbi = dbi.Where("network = ?", network)
	}
	if err := dbi.Find(&userStakings).Error; err != nil {
		return nil, err
	}
	return userStakings, nil
}

func GetUserStakingsOfNode(ctx context.Context, db *gorm.DB, nodeAddress string, network *string) ([]UserStaking, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStakings []UserStaking
	dbi := db.WithContext(dbCtx).Model(&UserStaking{}).Where("node_address = ?", nodeAddress).Where("valid = ?", true)
	if network != nil {
		dbi = dbi.Where("network = ?", network)
	}
	if err := dbi.Find(&userStakings).Error; err != nil {
		return nil, err
	}
	return userStakings, nil
}

func GetUserStaking(ctx context.Context, db *gorm.DB, userAddress, nodeAddress, network string) (*UserStaking, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStaking UserStaking
	if err := db.WithContext(dbCtx).Model(&UserStaking{}).Where("user_address = ?", userAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).Where("valid = ?", true).First(&userStaking).Error; err != nil {
		return nil, err
	}
	return &userStaking, nil
}