package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type GetRelayAccountBalanceInput struct {
	Address string `path:"address" validate:"required"`
}

type GetRelayAccountBalanceResponse struct {
	response.Response
	Data models.BigInt `json:"data"`
}

func GetRelayAccountBalance(c *gin.Context, in *GetRelayAccountBalanceInput) (*GetRelayAccountBalanceResponse, error) {
	balance, err := service.GetRelayAccountBalance(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &GetRelayAccountBalanceResponse{
		Data: models.BigInt{Int: *balance},
	}, nil
}
