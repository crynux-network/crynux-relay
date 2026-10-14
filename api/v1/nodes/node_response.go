package nodes

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/models"
)

type Node struct {
	Address            string                  `json:"address" gorm:"index"`
	Status             models.NodeStatus       `json:"status" gorm:"index"`
	GPUName            string                  `json:"gpu_name" gorm:"index"`
	GPUVram            uint64                  `json:"gpu_vram" gorm:"index"`
	QOSScore           uint64                  `json:"qos_score"`
	Version            string                  `json:"version"`
	ModelFiles         []models.ModelFileEntry `json:"model_files"`
	InMemoryModelFiles []models.ModelFileEntry `json:"in_memory_model_files"`
}

type NodeResponse struct {
	response.Response
	Data *Node `json:"data"`
}
