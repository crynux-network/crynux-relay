package models

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type ToEventType interface {
	ToEvent() (*Event, error)
}

type Event struct {
	gorm.Model
	Type             string `json:"type" gorm:"index"`
	NodeAddress      string `json:"node_address" gorm:"index"`
	TaskIDCommitment string `json:"task_id_commitment" gorm:"index"`
	Args             string `json:"args"`
}

func (e *Event) Save(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Save(e).Error; err != nil {
		return err
	}
	return nil
}

type TaskStartedEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
}

func (e *TaskStartedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskStarted",
		NodeAddress:      e.SelectedNode,
		TaskIDCommitment: e.TaskIDCommitment,
		Args:             string(bs),
	}, nil
}

type DownloadModelEvent struct {
	NodeAddress string   `json:"node_address"`
	ModelID     string   `json:"model_id"`
	TaskType    TaskType `json:"task_type"`
}

func (e *DownloadModelEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "DownloadModel",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}

type TaskScoreReadyEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
	Score            string `json:"score"`
}

func (e *TaskScoreReadyEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskScoreReady",
		NodeAddress:      e.SelectedNode,
		TaskIDCommitment: e.TaskIDCommitment,
		Args:             string(bs),
	}, nil

}

type TaskErrorReportedEvent struct {
	TaskIDCommitment string    `json:"task_id_commitment"`
	SelectedNode     string    `json:"selected_node"`
	TaskError        TaskError `json:"task_error"`
}

func (e *TaskErrorReportedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskErrorReported",
		NodeAddress:      e.SelectedNode,
		TaskIDCommitment: e.TaskIDCommitment,
		Args:             string(bs),
	}, nil
}

type TaskValidatedEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
}

func (e *TaskValidatedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskValidated",
		TaskIDCommitment: e.TaskIDCommitment,
		NodeAddress:      e.SelectedNode,
		Args:             string(bs),
	}, nil
}

type TaskEndInvalidatedEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
}

func (e *TaskEndInvalidatedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskEndInvalidated",
		TaskIDCommitment: e.TaskIDCommitment,
		NodeAddress:      e.SelectedNode,
		Args:             string(bs),
	}, nil
}

type TaskEndGroupRefundEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
}

func (e *TaskEndGroupRefundEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskEndGroupRefund",
		TaskIDCommitment: e.TaskIDCommitment,
		NodeAddress:      e.SelectedNode,
		Args:             string(bs),
	}, nil
}

type TaskEndAbortedEvent struct {
	TaskIDCommitment string          `json:"task_id_commitment"`
	AbortIssuer      string          `json:"abort_issuer"`
	LastStatus       TaskStatus      `json:"last_status"`
	AbortReason      TaskAbortReason `json:"abort_reason"`
}

func (e *TaskEndAbortedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskEndAborted",
		TaskIDCommitment: e.TaskIDCommitment,
		NodeAddress:      e.AbortIssuer,
		Args:             string(bs),
	}, nil
}

type TaskEndSuccessEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
}

func (e *TaskEndSuccessEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskEndSuccess",
		TaskIDCommitment: e.TaskIDCommitment,
		NodeAddress:      e.SelectedNode,
		Args:             string(bs),
	}, nil
}

type TaskEndGroupSuccessEvent struct {
	TaskIDCommitment string `json:"task_id_commitment"`
	SelectedNode     string `json:"selected_node"`
}

func (e *TaskEndGroupSuccessEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "TaskEndGroupSuccess",
		TaskIDCommitment: e.TaskIDCommitment,
		NodeAddress:      e.SelectedNode,
		Args:             string(bs),
	}, nil
}

type NodeKickedOutEvent struct {
	NodeAddress      string `json:"node_address"`
	TaskIDCommitment string `json:"task_id_commitment"`
}

func (e *NodeKickedOutEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "NodeKickedOut",
		NodeAddress:      e.NodeAddress,
		TaskIDCommitment: e.TaskIDCommitment,
		Args:             string(bs),
	}, nil
}

type NodeSlashedEvent struct {
	NodeAddress      string `json:"node_address"`
	TaskIDCommitment string `json:"task_id_commitment"`
}

func (e *NodeSlashedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:             "NodeSlashed",
		NodeAddress:      e.NodeAddress,
		TaskIDCommitment: e.TaskIDCommitment,
		Args:             string(bs),
	}, nil
}

type NodeJoinEvent struct {
	NodeAddress string `json:"node_address"`
}

func (e *NodeJoinEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "NodeJoin",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}

type NodeQuitEvent struct {
	NodeAddress             string `json:"node_address"`
	BlockchainTransactionID uint   `json:"blockchain_transaction_id"`
}

func (e *NodeQuitEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "NodeQuit",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}

type NodeStakingEvent struct {
	NodeAddress   string `json:"node_address"`
	StakingAmount BigInt `json:"staking_amount"`
}

func (e *NodeStakingEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "NodeStaking",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}

type UserStakingEvent struct {
	UserAddress string `json:"user_address"`
	NodeAddress string `json:"node_address"`
	Amount      BigInt `json:"amount"`
	Network     string `json:"network"`
}

func (e *UserStakingEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "UserStaking",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}

type UserUnstakingEvent struct {
	UserAddress string `json:"user_address"`
	NodeAddress string `json:"node_address"`
	Amount      BigInt `json:"amount"`
	Network     string `json:"network"`
}

func (e *UserUnstakingEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "UserUnstaking",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}

type NodeDelegatorShareChangedEvent struct {
	NodeAddress string `json:"node_address"`
	Share       uint8  `json:"share"`
	Network     string `json:"network"`
}

func (e *NodeDelegatorShareChangedEvent) ToEvent() (*Event, error) {
	bs, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type:        "NodeDelegatorShareChanged",
		NodeAddress: e.NodeAddress,
		Args:        string(bs),
	}, nil
}
