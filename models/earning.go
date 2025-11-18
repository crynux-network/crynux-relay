package models

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type UserStakingEarning struct {
	gorm.Model
	UserAddress string       `json:"user_address" gorm:"uniqueIndex:idx_user_node_network_time;type:string;size:191;not null"`
	NodeAddress string       `json:"node_address" gorm:"uniqueIndex:idx_user_node_network_time;type:string;size:191;not null"`
	Network     string       `json:"network" gorm:"uniqueIndex:idx_user_node_network_time;type:string;size:191;not null"`
	Earning     BigInt       `json:"earning" gorm:"type:string;size:191;not null"`
	Time        sql.NullTime `json:"time" gorm:"uniqueIndex:idx_user_node_network_time"`
}

type NodeEarning struct {
	gorm.Model
	NodeAddress      string       `json:"node_address" gorm:"uniqueIndex:idx_node_time;type:string;size:191;not null"`
	OperatorEarning  BigInt       `json:"operator_earning" gorm:"type:string;size:191;not null"`
	DelegatorEarning BigInt       `json:"delegator_earning" gorm:"type:string;size:191;not null"`
	Time             sql.NullTime `json:"time" gorm:"uniqueIndex:idx_node_time"`
}

type UserEarning struct {
	gorm.Model
	UserAddress string       `json:"node_address" gorm:"uniqueIndex:idx_user_time;type:string;size:191;not null"`
	Earning     BigInt       `json:"earning" gorm:"type:string;size:191;not null"`
	Time        sql.NullTime `json:"time" gorm:"uniqueIndex:idx_user_time"`
}

func GetTotalNodeEarning(ctx context.Context, db *gorm.DB, nodeAddress string) (*NodeEarning, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodeEarning NodeEarning
	if err := db.WithContext(dbCtx).Model(&NodeEarning{}).Where("node_address = ?", nodeAddress).Where("time IS NULL").First(&nodeEarning).Error; err != nil {
		return nil, err
	}
	return &nodeEarning, nil
}

func GetNodeEarnings(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) ([]NodeEarning, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodeEarnings []NodeEarning
	if err := db.WithContext(dbCtx).Model(&NodeEarning{}).Where("node_address = ?", nodeAddress).Where("time >= ?", start).Where("time < ?", end).Order("time desc").Find(&nodeEarnings).Error; err != nil {
		return nil, err
	}

	return nodeEarnings, nil
}

func GetTotalUserEarning(ctx context.Context, db *gorm.DB, userAddress string) (*UserEarning, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userEarning UserEarning
	if err := db.WithContext(dbCtx).Model(&UserEarning{}).Where("user_address = ?", userAddress).Where("time IS NULL").First(&userEarning).Error; err != nil {
		return nil, err
	}
	return &userEarning, nil
}

func GetUserEarnings(ctx context.Context, db *gorm.DB, userAddress string, start, end time.Time) ([]UserEarning, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userEarnings []UserEarning
	if err := db.WithContext(dbCtx).Model(&UserEarning{}).Where("user_address = ?", userAddress).Where("time >= ?", start).Where("time < ?", end).Order("time desc").Find(&userEarnings).Error; err != nil {
		return nil, err
	}

	return userEarnings, nil
}

func GetTotalUserStakingEarning(ctx context.Context, db *gorm.DB, userAddress, nodeAddress, network string) (*UserStakingEarning, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStakingEarning UserStakingEarning
	if err := db.WithContext(dbCtx).Model(&UserStakingEarning{}).Where("user_address = ?", userAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).Where("time IS NULL").First(&userStakingEarning).Error; err != nil {
		return nil, err
	}
	return &userStakingEarning, nil
}

func GetUserStakingEarnings(ctx context.Context, db *gorm.DB, userAddress, nodeAddress, network string, start, end time.Time) ([]UserStakingEarning, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStakingEarnings []UserStakingEarning
	if err := db.WithContext(dbCtx).Model(&UserStakingEarning{}).Where("user_address = ?", userAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).Where("time >= ?", start).Where("time < ?", end).Order("time desc").Find(&userStakingEarnings).Error; err != nil {
		return nil, err
	}

	return userStakingEarnings, nil
}
