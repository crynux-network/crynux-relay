package taskfee

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type GetTaskFeeInput struct {
	Address string `path:"address" json:"address" description:"Address of account"`
}

type GetTaskFeeResponse struct {
	response.Response
	Data models.BigInt `json:"data"`
}

func GetTaskFee(c *gin.Context, in *GetTaskFeeInput) (*GetTaskFeeResponse, error) {
	taskFee, err := service.GetTaskFee(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		return nil, err
	}
	return &GetTaskFeeResponse{
		Data: models.BigInt{Int: *taskFee},
	}, nil
}
