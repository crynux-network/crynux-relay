package relayaccount

import (
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type GetBalanceSigningInput struct {
	Address string `path:"address" json:"address" description:"Address of account"`
}

type GetBalanceInput struct {
	GetBalanceSigningInput
	Timestamp *int64 `query:"timestamp" json:"timestamp" description:"Signature timestamp"`
	Signature string `query:"signature" json:"signature" description:"Signature"`
}

type GetBalanceResponse struct {
	response.Response
	Data models.BigInt `json:"data"`
}

func GetBalance(c *gin.Context, in *GetBalanceInput) (*GetBalanceResponse, error) {
	if middleware.GetUserAddress(c) != in.Address {
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
