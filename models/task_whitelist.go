package models

import "time"

type TaskWhitelist struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Address   string    `json:"address" gorm:"not null;size:42;uniqueIndex:idx_task_whitelist_address"`
}
