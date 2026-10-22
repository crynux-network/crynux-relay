package models

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	dbmodels "crynux_relay/models"
	"crynux_relay/service"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

var getDB = config.GetDB

type LoadedModelData struct {
	ModelID            string                   `json:"model_id"`
	ModelType          dbmodels.LoadedModelType `json:"model_type"`
	ExecutionDType     string                   `json:"execution_dtype"`
	QuantizationConfig json.RawMessage          `json:"quantization_config"`
	ModelCacheHash     string                   `json:"model_cache_hash"`
	ModelCacheConfig   json.RawMessage          `json:"model_cache_config"`
	MinVRAM            uint64                   `json:"min_vram"`
	InMemoryNodeCount  int64                    `json:"in_memory_node_count"`
	OnDiskNodeCount    int64                    `json:"on_disk_node_count"`
}

type GetLoadedModelsResponse struct {
	response.Response
	Data []LoadedModelData `json:"data"`
}

func GetLoadedModels(c *gin.Context) (*GetLoadedModelsResponse, error) {
	views, err := service.GetLoadedModelViews(c.Request.Context(), getDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	data := make([]LoadedModelData, 0, len(views))
	for _, view := range views {
		cacheHash, err := dbmodels.Hash32FromBytes(view.Model.ModelCacheHash)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
		data = append(data, LoadedModelData{
			ModelID:            view.Model.ModelID,
			ModelType:          view.Model.ModelType,
			ExecutionDType:     view.Model.ExecutionDType,
			QuantizationConfig: json.RawMessage(view.Model.QuantizationConfig),
			ModelCacheHash:     cacheHash.Hex(),
			ModelCacheConfig:   json.RawMessage(view.Model.ModelCacheConfig),
			MinVRAM:            view.Model.MinVRAM,
			InMemoryNodeCount:  view.InMemoryNodeCount,
			OnDiskNodeCount:    view.OnDiskNodeCount,
		})
	}
	return &GetLoadedModelsResponse{
		Data: data,
	}, nil
}
