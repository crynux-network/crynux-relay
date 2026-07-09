package models

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

var getDB = config.GetDB

type LoadedModelData struct {
	ModelID string `json:"model_id"`
	MinVRAM uint64 `json:"min_vram"`
}

type GetLoadedModelsResponse struct {
	response.Response
	Data []LoadedModelData `json:"data"`
}

func GetLoadedModels(c *gin.Context) (*GetLoadedModelsResponse, error) {
	loadedModels, err := service.ListLoadedModels(c.Request.Context(), getDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	data := make([]LoadedModelData, 0, len(loadedModels))
	for _, loadedModel := range loadedModels {
		data = append(data, LoadedModelData{
			ModelID: loadedModel.ModelID,
			MinVRAM: loadedModel.MinVRAM,
		})
	}
	return &GetLoadedModelsResponse{
		Data: data,
	}, nil
}
