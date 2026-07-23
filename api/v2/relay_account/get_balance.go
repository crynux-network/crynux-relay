package relayaccount

import (
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type GetBalanceInput struct {
	Address string `path:"address" json:"address" description:"Address of account"`
}

type GetBalanceResponse struct {
	response.Response
	Data models.BigInt `json:"data"`
}

func GetBalance(c *gin.Context, in *GetBalanceInput) (*GetBalanceResponse, error) {
	address := middleware.GetUserAddress(c)
	if address != in.Address {
		return nil, response.NewValidationErrorResponse("address", "Address mismatch")
	}

	balance, err := service.GetRelayAccountBalance(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &GetBalanceResponse{
		Data: models.BigInt{Int: *balance},
	}, nil
}
