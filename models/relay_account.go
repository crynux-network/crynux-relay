package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type RelayAccount struct {
	gorm.Model
	Address string `json:"address" gorm:"uniqueIndex"`
	Balance BigInt `json:"balance" gorm:"type:string;size:255"`
}

type RelayAccountEventStatus int8

const (
	RelayAccountEventStatusPending RelayAccountEventStatus = iota
	RelayAccountEventStatusProcessed
	RelayAccountEventStatusInvalid
)

type RelayAccountEventType int8

const (
	RelayAccountEventTypeTaskIncome        RelayAccountEventType = 0
	RelayAccountEventTypeDaoTaskShare      RelayAccountEventType = 1
	RelayAccountEventTypeWithdrawFeeIncome RelayAccountEventType = 2
	RelayAccountEventTypeDeposit           RelayAccountEventType = 3
	RelayAccountEventTypeTaskPayment       RelayAccountEventType = 4
	RelayAccountEventTypeTaskRefund        RelayAccountEventType = 5
	RelayAccountEventTypeWithdraw          RelayAccountEventType = 6
	RelayAccountEventTypeWithdrawRefund    RelayAccountEventType = 7
	RelayAccountEventTypeUserDelegation    RelayAccountEventType = 8
)

type RelayAccountEvent struct {
	ID        uint                    `json:"id" gorm:"primarykey"`
	CreatedAt time.Time               `json:"created_at" gorm:"not null"`
	Address   string                  `json:"address" gorm:"not null;index"`
	Amount    BigInt                  `json:"amount" gorm:"not null"`
	Status    RelayAccountEventStatus `json:"status" gorm:"not null;default:0;index"`
	Reason    string                  `json:"reason" gorm:"not null;uniqueIndex"`
	Type      RelayAccountEventType   `json:"type" gorm:"not null;index"`
	MAC       string                  `json:"mac" gorm:"not null"`
}

func GetRelayAccounts(ctx context.Context, db *gorm.DB, addresses []string) ([]RelayAccount, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var accounts []RelayAccount
	if err := db.WithContext(dbCtx).Where("address IN (?)", addresses).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func GetLastProcessedRelayAccountEventID(ctx context.Context, db *gorm.DB) (uint, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var event RelayAccountEvent
	if err := db.WithContext(dbCtx).Where("status = ?", RelayAccountEventStatusProcessed).Last(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return event.ID, nil
}

func GetRelayAccountDepositEvent(ctx context.Context, db *gorm.DB, txHash, network string) (*RelayAccountEvent, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var event RelayAccountEvent
	reason := fmt.Sprintf("%d-%s-%s", RelayAccountEventTypeDeposit, txHash, network)
	if err := db.WithContext(dbCtx).Where("reason = ?", reason).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}
