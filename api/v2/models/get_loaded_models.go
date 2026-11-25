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
	DType              string                   `json:"dtype"`
	QuantizationConfig json.RawMessage          `json:"quantization_config"`
	Components         json.RawMessage          `json:"components,omitempty"`
	Lora               json.RawMessage          `json:"lora,omitempty"`
	Loras              json.RawMessage          `json:"loras,omitempty"`
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
		item := LoadedModelData{
			ModelID:            view.Model.ModelID,
			ModelType:          view.Model.ModelType,
			DType:              view.Model.DType,
			QuantizationConfig: json.RawMessage(view.Model.QuantizationConfig),
			MinVRAM:            view.Model.MinVRAM,
			InMemoryNodeCount:  view.InMemoryNodeCount,
			OnDiskNodeCount:    view.OnDiskNodeCount,
		}
		switch view.Model.ModelType {
		case dbmodels.LoadedModelTypeSD:
			components := view.Public.Components
			if components == nil {
				components = map[string]any{}
			}
			item.Components = mustJSON(components)
			if view.Public.Lora == nil {
				item.Lora = json.RawMessage("null")
			} else {
				item.Lora = mustJSON(view.Public.Lora)
			}
		case dbmodels.LoadedModelTypeVideo:
			components := view.Public.Components
			if components == nil {
				components = map[string]any{}
			}
			item.Components = mustJSON(components)
			loras := view.Public.Loras
			if loras == nil {
				loras = []any{}
			}
			item.Loras = mustJSON(loras)
		}
		data = append(data, item)
	}
	return &GetLoadedModelsResponse{
		Data: data,
	}, nil
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}
