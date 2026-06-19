package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type PendingSlashStatus string

const (
	PendingSlashStatusPending PendingSlashStatus = "pending"
	PendingSlashStatusSlashed PendingSlashStatus = "slashed"
)

type SlashEvidence struct {
	TaskSnapshots     []SlashEvidenceTaskSnapshot    `json:"task_snapshots"`
	NodeSnapshots     []SlashEvidenceNodeSnapshot    `json:"node_snapshots"`
	ValidationContext SlashEvidenceValidationContext `json:"validation_context"`
	InputArtifacts    []SlashEvidenceArtifacts       `json:"input_artifacts"`
	ResultArtifacts   []SlashEvidenceArtifacts       `json:"result_artifacts"`
	IncompleteReason  string                         `json:"incomplete_reason,omitempty"`
}

type SlashEvidenceTaskSnapshot struct {
	TaskIDCommitment string     `json:"task_id_commitment"`
	TaskID           string     `json:"task_id"`
	TaskArgs         string     `json:"task_args"`
	TaskType         TaskType   `json:"task_type"`
	TaskVersion      string     `json:"task_version"`
	Creator          string     `json:"creator"`
	TaskFee          string     `json:"task_fee"`
	Score            string     `json:"score"`
	QOSScore         *int64     `json:"qos_score,omitempty"`
	ModelIDs         []string   `json:"model_ids"`
	CreateTime       *time.Time `json:"create_time,omitempty"`
	StartTime        *time.Time `json:"start_time,omitempty"`
	ScoreReadyTime   *time.Time `json:"score_ready_time,omitempty"`
	ValidatedTime    *time.Time `json:"validated_time,omitempty"`
}

type SlashEvidenceNodeSnapshot struct {
	Address                string                   `json:"address"`
	Network                string                   `json:"network"`
	Status                 NodeStatus               `json:"status"`
	GPUName                string                   `json:"gpu_name"`
	GPUVram                uint64                   `json:"gpu_vram"`
	MajorVersion           uint64                   `json:"major_version"`
	MinorVersion           uint64                   `json:"minor_version"`
	PatchVersion           uint64                   `json:"patch_version"`
	OperatorStakeAmount    string                   `json:"operator_stake_amount"`
	QOSScore               float64                  `json:"qos_score"`
	HealthBase             float64                  `json:"health_base"`
	HealthUpdatedAt        *time.Time               `json:"health_updated_at,omitempty"`
	DelegatorCount         int64                    `json:"delegator_count"`
	DelegatedStakingAmount string                   `json:"delegated_staking_amount"`
	Models                 []SlashEvidenceNodeModel `json:"models"`
}

type SlashEvidenceNodeModel struct {
	ModelID string `json:"model_id"`
	InUse   bool   `json:"in_use"`
}

type SlashEvidenceValidationContext struct {
	Reason            string   `json:"reason"`
	TaskID            string   `json:"task_id"`
	GroupTaskIDs      []string `json:"group_task_ids"`
	TaskIDCommitments []string `json:"task_id_commitments"`
}

type SlashEvidenceArtifacts struct {
	TaskIDCommitment string   `json:"task_id_commitment"`
	SourcePath       string   `json:"source_path,omitempty"`
	StoredPath       string   `json:"stored_path,omitempty"`
	Files            []string `json:"files,omitempty"`
	Status           string   `json:"status"`
}

type PendingSlash struct {
	gorm.Model
	Status           PendingSlashStatus `json:"status" gorm:"not null;size:32;index"`
	NodeAddress      string             `json:"node_address" gorm:"not null;size:191;index"`
	Network          string             `json:"network" gorm:"not null;size:191;index"`
	TaskIDCommitment string             `json:"task_id_commitment" gorm:"not null;size:191;index"`
	EvidenceJSON     string             `json:"evidence_json" gorm:"type:longtext;not null"`
	EvidenceComplete bool               `json:"evidence_complete" gorm:"not null;index"`
}

func (pendingSlash *PendingSlash) Create(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Create(pendingSlash).Error
}

func (pendingSlash *PendingSlash) Save(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Save(pendingSlash).Error
}

func GetPendingSlashByID(ctx context.Context, db *gorm.DB, id uint) (*PendingSlash, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var pendingSlash PendingSlash
	if err := db.WithContext(dbCtx).First(&pendingSlash, id).Error; err != nil {
		return nil, err
	}
	return &pendingSlash, nil
}
