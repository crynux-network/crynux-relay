package models

import (
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

type WithdrawRecord struct {
	gorm.Model
	Address             string              `json:"address" gorm:"not null;index"`
	BenefitAddress      string              `json:"benefit_address" gorm:"not null;index"`
	Amount              BigInt              `json:"amount" gorm:"not null"`
	Network             string              `json:"network" gorm:"not null;index"`
	Status              WithdrawStatus      `json:"status" gorm:"not null;default:0;index"`
	LocalStatus         WithdrawLocalStatus `json:"local_status" gorm:"not null;default:0;index"`
	RelayAccountEventID uint                `json:"relay_account_event_id" gorm:"not null;default:0"`
	TxHash              sql.NullString      `json:"tx_hash" gorm:"null;"`
	WithdrawalFee       BigInt              `json:"withdrawal_fee" gorm:"not null"`
	Timestamp           sql.NullInt64       `json:"timestamp" gorm:"null"`
	Signature           sql.NullString      `json:"signature" gorm:"type:varchar(255);null"`
	MAC                 string              `json:"mac" gorm:"not null"`
}

func (r *WithdrawRecord) MACString() string {
	return fmt.Sprintf("%d-%s-%s-%s-%s", r.ID, r.Address, r.BenefitAddress, (&r.Amount).String(), r.Network)
}

type WithdrawStatus int8

const (
	WithdrawStatusPending WithdrawStatus = iota
	WithdrawStatusSuccess
	WithdrawStatusFailed
)

type WithdrawLocalStatus int8

const (
	WithdrawLocalStatusPending WithdrawLocalStatus = iota
	WithdrawLocalStatusProcessed
	WithdrawLocalStatusInvalid
)
