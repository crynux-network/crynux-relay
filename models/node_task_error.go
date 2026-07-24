package models

import "time"

type NodeTaskError struct {
	ID               uint      `json:"id" gorm:"primarykey;index:idx_node_task_errors_created_id,priority:2"`
	CreatedAt        time.Time `json:"created_at" gorm:"index:idx_node_task_errors_created_id,priority:1"`
	UpdatedAt        time.Time `json:"updated_at"`
	NodeAddress      string    `json:"node_address" gorm:"type:string;size:42;not null;index;uniqueIndex:idx_node_task_errors_node_task,priority:1"`
	TaskIDCommitment string    `json:"task_id_commitment" gorm:"type:string;size:191;not null;index;uniqueIndex:idx_node_task_errors_node_task,priority:2"`
	TaskArgs         string    `json:"task_args" gorm:"type:longtext;not null"`
	ErrorType        string    `json:"error_type" gorm:"type:string;size:64;not null"`
	Message          string    `json:"message" gorm:"type:longtext;not null"`
	StackTrace       string    `json:"stack_trace" gorm:"type:longtext;not null"`
	CapturedAt       int64     `json:"captured_at" gorm:"not null"`
}
