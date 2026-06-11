package models

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type DelegatedSlashJobStatus string

const (
	DelegatedSlashJobStatusPending    DelegatedSlashJobStatus = "pending"
	DelegatedSlashJobStatusProcessing DelegatedSlashJobStatus = "processing"
	DelegatedSlashJobStatusCompleted  DelegatedSlashJobStatus = "completed"
	DelegatedSlashJobStatusFailed     DelegatedSlashJobStatus = "failed"
)

type DelegatedSlashJob struct {
	gorm.Model
	NodeAddress              string                  `json:"node_address" gorm:"not null;size:191;uniqueIndex:idx_delegated_slash_job_node_network"`
	Network                  string                  `json:"network" gorm:"not null;size:191;uniqueIndex:idx_delegated_slash_job_node_network;index"`
	Status                   DelegatedSlashJobStatus `json:"status" gorm:"not null;size:32;index"`
	LatestBatchTransactionID sql.NullInt64           `json:"latest_batch_transaction_id" gorm:"index"`
	LastError                sql.NullString          `json:"last_error"`
}

type DelegatedStakingSlashRecord struct {
	gorm.Model
	SlashJobID       sql.NullInt64 `json:"slash_job_id" gorm:"index"`
	NodeAddress      string        `json:"node_address" gorm:"not null;size:191;index:idx_delegated_staking_slash_record_node_delegator"`
	DelegatorAddress string        `json:"delegator_address" gorm:"not null;size:191;index:idx_delegated_staking_slash_record_node_delegator"`
	Network          string        `json:"network" gorm:"not null;size:191;index;uniqueIndex:idx_delegated_staking_slash_record_event;index:idx_delegated_staking_slash_record_node_delegator"`
	Amount           BigInt        `json:"amount" gorm:"type:string;size:191;not null"`
	SlashTxHash      string        `json:"slash_tx_hash" gorm:"not null;size:191;uniqueIndex:idx_delegated_staking_slash_record_event"`
	BlockNumber      uint64        `json:"block_number" gorm:"not null;index"`
	LogIndex         uint          `json:"log_index" gorm:"not null;uniqueIndex:idx_delegated_staking_slash_record_event"`
}

func GetDelegatedSlashJob(ctx context.Context, db *gorm.DB, nodeAddress, network string) (*DelegatedSlashJob, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var job DelegatedSlashJob
	if err := db.WithContext(dbCtx).Where("node_address = ? AND network = ?", nodeAddress, network).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func HasUnfinishedDelegatedSlashJobForNode(ctx context.Context, db *gorm.DB, nodeAddress string) (bool, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int64
	if err := db.WithContext(dbCtx).
		Model(&DelegatedSlashJob{}).
		Where("node_address = ? AND status <> ?", nodeAddress, DelegatedSlashJobStatusCompleted).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
