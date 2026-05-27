package models

import "gorm.io/gorm"

type NodeNameCount struct {
	gorm.Model
	GPUName     string `json:"gpu_name" gorm:"not null;size:191;uniqueIndex:idx_node_name_count_unique"`
	GPUVram     uint64 `json:"gpu_vram" gorm:"not null;uniqueIndex:idx_node_name_count_unique"`
	NodeVersion string `json:"node_version" gorm:"not null;size:32;uniqueIndex:idx_node_name_count_unique"`
	ActiveCount uint64 `json:"active_count" gorm:"not null"`
}
