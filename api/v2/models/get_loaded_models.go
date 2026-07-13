package models

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	dbmodels "crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

var getDB = config.GetDB

type LoadedModelData struct {
	ModelID           string                   `json:"model_id"`
	ModelType         dbmodels.LoadedModelType `json:"model_type"`
	MinVRAM           uint64                   `json:"min_vram"`
	InMemoryNodeCount int64                    `json:"in_memory_node_count"`
	OnDiskNodeCount   int64                    `json:"on_disk_node_count"`
}

type GetLoadedModelsResponse struct {
	response.Response
	Data []LoadedModelData `json:"data"`
}

func GetLoadedModels(c *gin.Context) (*GetLoadedModelsResponse, error) {
	loadedModels, err := dbmodels.ListLoadedModels(c.Request.Context(), getDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	nodeCounts, err := service.GetLoadedModelNodeCounts(c.Request.Context(), getDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	data := make([]LoadedModelData, 0, len(loadedModels))
	for _, loadedModel := range loadedModels {
		nodeCount := nodeCounts[loadedModel.ModelID]
		data = append(data, LoadedModelData{
			ModelID:           loadedModel.ModelID,
			ModelType:         loadedModel.ModelType,
			MinVRAM:           loadedModel.MinVRAM,
			InMemoryNodeCount: nodeCount.InMemory,
			OnDiskNodeCount:   nodeCount.OnDisk,
		})
	}
	return &GetLoadedModelsResponse{
		Data: data,
	}, nil
}
