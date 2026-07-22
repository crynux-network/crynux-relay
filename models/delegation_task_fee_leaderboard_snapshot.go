package models

import "time"

type DelegationTaskFeeLeaderboardSnapshot struct {
	ID               uint `gorm:"primarykey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Rank             uint8   `gorm:"column:leaderboard_rank;not null;index"`
	DelegatorAddress string  `gorm:"type:string;size:191;not null"`
	NodeAddress      string  `gorm:"type:string;size:191;not null"`
	Network          string  `gorm:"type:string;size:64;not null"`
	StakingAmount    BigInt  `gorm:"type:decimal(65,0);not null;default:0"`
	TaskFee          BigInt  `gorm:"type:decimal(65,0);not null;default:0"`
	DelegationApr12m float64 `gorm:"column:delegation_apr_12m;not null;default:0"`
}
