package relayaccount

import (
	"crynux_relay/api/v1/middleware"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"time"

	"github.com/gin-gonic/gin"
)

type GetLockedVestingInput struct {
	Address string `path:"address" json:"address" description:"Address of account"`
}

type GetLockedVestingResponse struct {
	response.Response
	Data models.BigInt `json:"data"`
}

func GetLockedVesting(c *gin.Context, in *GetLockedVestingInput) (*GetLockedVestingResponse, error) {
	address := middleware.GetUserAddress(c)
	if address != in.Address {
		validationErr := response.NewValidationErrorResponse("address", "Address mismatch")
		return nil, validationErr
	}

	lockedAmount, err := service.GetAddressLockedVestingAmount(c.Request.Context(), config.GetDB(), in.Address, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &GetLockedVestingResponse{
		Data: models.BigInt{Int: *lockedAmount},
	}, nil
}
