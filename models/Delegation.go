package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Delegation struct {
	gorm.Model
	DelegatorAddress string `json:"delegator_address" gorm:"index;type:string;size:191;not null"`
	NodeAddress      string `json:"node_address" gorm:"index;type:string;size:191;not null"`
	Amount           BigInt `json:"amount" gorm:"type:string;size:191;not null"`
	Valid            bool   `json:"valid" gorm:"not null;index"`
	Network          string `json:"network" gorm:"not null;index;type:string;size:191"`
}

func GetDelegation(ctx context.Context, db *gorm.DB, delegatorAddress, nodeAddress, network string) (*Delegation, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStaking Delegation
	if err := db.WithContext(dbCtx).Model(&Delegation{}).Where("delegator_address = ?", delegatorAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).Where("valid = ?", true).First(&userStaking).Error; err != nil {
		return nil, err
	}
	return &userStaking, nil
}
