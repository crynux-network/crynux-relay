package models

import "time"

type VestingDelegationEmissionDetail struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	VestingRecordID  uint      `json:"vesting_record_id" gorm:"not null;index:idx_vded_vesting_record_id"`
	UserAddress      string    `json:"user_address" gorm:"not null;size:64;index:idx_vded_user_node_network_start,priority:1"`
	NodeAddress      string    `json:"node_address" gorm:"not null;size:64;index:idx_vded_node_network_start,priority:1;index:idx_vded_user_node_network_start,priority:2"`
	Network          string    `json:"network" gorm:"not null;size:64;index:idx_vded_node_network_start,priority:2;index:idx_vded_user_node_network_start,priority:3"`
	TaskFee          BigInt    `json:"task_fee" gorm:"not null;type:string;size:191"`
	EmissionAmount   BigInt    `json:"emission_amount" gorm:"not null;type:string;size:191"`
	Source           string    `json:"source" gorm:"not null;size:64;uniqueIndex:idx_vded_source_detail_external_id,priority:1"`
	DetailExternalID string    `json:"detail_external_id" gorm:"not null;size:191;uniqueIndex:idx_vded_source_detail_external_id,priority:2"`
	StartTime        time.Time `json:"start_time" gorm:"not null;index:idx_vded_node_network_start,priority:3;index:idx_vded_user_node_network_start,priority:4"`
}
