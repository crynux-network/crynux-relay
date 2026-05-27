package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
)

type AddNodeNameWhitelistInput struct {
	GPUName     string `json:"gpu_name" validate:"required"`
	GPUVram     uint64 `json:"gpu_vram" validate:"required"`
	NodeVersion string `json:"node_version" validate:"required"`
}

type DeleteNodeNameWhitelistInput struct {
	GPUName     string `path:"gpu_name" validate:"required"`
	GPUVram     uint64 `path:"gpu_vram" validate:"required"`
	NodeVersion string `path:"node_version" validate:"required"`
}

type ListNodeNameWhitelistData struct {
	Entries []service.NodeNameWhitelistEntry `json:"entries"`
}

type ListNodeNameWhitelistResponse struct {
	response.Response
	Data ListNodeNameWhitelistData `json:"data"`
}

func ListNodeNameWhitelist(c *gin.Context) (*ListNodeNameWhitelistResponse, error) {
	entries, err := service.ListNodeNameWhitelistFromCache(c.Request.Context(), config.GetDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListNodeNameWhitelistResponse{
		Data: ListNodeNameWhitelistData{
			Entries: entries,
		},
	}, nil
}

func AddNodeNameWhitelist(c *gin.Context, in *AddNodeNameWhitelistInput) (*response.Response, error) {
	err := service.AddNodeNameWhitelist(c.Request.Context(), config.GetDB(), in.GPUName, in.GPUVram, in.NodeVersion)
	if err != nil {
		if errors.Is(err, service.ErrInvalidNodeNameGPUName) ||
			errors.Is(err, service.ErrInvalidNodeNameVersion) ||
			errors.Is(err, service.ErrNodeNameWhitelistExists) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}

func DeleteNodeNameWhitelist(c *gin.Context, in *DeleteNodeNameWhitelistInput) (*response.Response, error) {
	err := service.DeleteNodeNameWhitelist(c.Request.Context(), config.GetDB(), in.GPUName, in.GPUVram, in.NodeVersion)
	if err != nil {
		if errors.Is(err, service.ErrInvalidNodeNameGPUName) ||
			errors.Is(err, service.ErrInvalidNodeNameVersion) ||
			errors.Is(err, service.ErrNodeNameWhitelistMissing) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
