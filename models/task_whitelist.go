package models

import "gorm.io/gorm"

type TaskWhitelist struct {
	gorm.Model
	Address string `json:"address" gorm:"not null;size:42;uniqueIndex:idx_task_whitelist_address"`
}
