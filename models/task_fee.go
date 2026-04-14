package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type TaskFee struct {
	gorm.Model
	Address string `json:"address" gorm:"uniqueIndex"`
	TaskFee BigInt `json:"task_fee" gorm:"type:string;size:255"`
}

type TaskFeeEventStatus int8

const (
	TaskFeeEventStatusPending TaskFeeEventStatus = iota
	TaskFeeEventStatusProcessed
	TaskFeeEventStatusInvalid
)

type TaskFeeEventType int8

const (
	TaskFeeEventTypeTask TaskFeeEventType = iota
	TaskFeeEventTypeDraw
	TaskFeeEventTypeWithdrawalFee
	TaskFeeEventTypeBought
	TaskFeeEventTypeUserCommission
)

type TaskFeeEvent struct {
	ID        uint               `json:"id" gorm:"primarykey"`
	CreatedAt time.Time          `json:"created_at" gorm:"not null"`
	Address   string             `json:"address" gorm:"not null;index"`
	TaskFee   BigInt             `json:"task_fee" gorm:"not null"`
	Status    TaskFeeEventStatus `json:"status" gorm:"not null;default:0;index"`
	Reason    string             `json:"reason" gorm:"not null;uniqueIndex"`
	Type      TaskFeeEventType   `json:"type" gorm:"not null;index"`
	MAC       string             `json:"mac" gorm:"not null"`
}

func GetTaskFees(ctx context.Context, db *gorm.DB, addresses []string) ([]TaskFee, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var taskFees []TaskFee
	if err := db.WithContext(dbCtx).Where("address IN (?)", addresses).Find(&taskFees).Error; err != nil {
		return nil, err
	}
	return taskFees, nil
}

func GetLastProcessedTaskFeeEventID(ctx context.Context, db *gorm.DB) (uint, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var event TaskFeeEvent
	if err := db.WithContext(dbCtx).Where("status = ?", TaskFeeEventStatusProcessed).Last(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return event.ID, nil
}

func GetTaskFeeBoughtEvent(ctx context.Context, db *gorm.DB, txHash, network string) (*TaskFeeEvent, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var event TaskFeeEvent
	if err := db.WithContext(dbCtx).Where("reason = ?", fmt.Sprintf("%d-%s-%s", TaskFeeEventTypeBought, txHash, network)).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil

}
