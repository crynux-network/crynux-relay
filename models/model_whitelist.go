package models

import "time"

type ModelWhitelist struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ModelName string    `json:"model_name" gorm:"not null;size:512;uniqueIndex:idx_model_whitelist_model_name"`
}
