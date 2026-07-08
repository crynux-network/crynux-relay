package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type VestingDelegationEmissionDetail struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	VestingRecordID uint      `json:"vesting_record_id" gorm:"not null;index:idx_vded_vesting_record_id"`
	UserAddress     string    `json:"user_address" gorm:"not null;size:64;uniqueIndex:idx_vded_user_node_network_start,priority:1"`
	NodeAddress     string    `json:"node_address" gorm:"not null;size:64;index:idx_vded_node_network_start,priority:1;uniqueIndex:idx_vded_user_node_network_start,priority:2"`
	Network         string    `json:"network" gorm:"not null;size:64;index:idx_vded_node_network_start,priority:2;uniqueIndex:idx_vded_user_node_network_start,priority:3"`
	TaskFee         BigInt    `json:"task_fee" gorm:"not null;type:string;size:191"`
	EmissionAmount  BigInt    `json:"emission_amount" gorm:"not null;type:string;size:191"`
	StartTime       time.Time `json:"start_time" gorm:"not null;index:idx_vded_node_network_start,priority:3;uniqueIndex:idx_vded_user_node_network_start,priority:4"`
}

func ListVestingDelegationEmissionDetailsByUserNodeNetworkAndStartTimeRange(ctx context.Context, db *gorm.DB, userAddress, nodeAddress, network string, startTime, endTime time.Time) ([]VestingDelegationEmissionDetail, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var details []VestingDelegationEmissionDetail
	if err := db.WithContext(dbCtx).
		Model(&VestingDelegationEmissionDetail{}).
		Where("user_address = ?", userAddress).
		Where("node_address = ?", nodeAddress).
		Where("network = ?", network).
		Where("start_time >= ? AND start_time < ?", startTime, endTime).
		Order("start_time ASC").
		Find(&details).Error; err != nil {
		return nil, err
	}
	return details, nil
}
