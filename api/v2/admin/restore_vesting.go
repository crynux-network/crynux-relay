package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

type RestoreVestingInput struct {
	NodeAddress string `json:"node_address" validate:"required"`
}

type RestoreVestingResponse struct {
	response.Response
}

func RestoreNodeVestings(c *gin.Context, in *RestoreVestingInput) (*RestoreVestingResponse, error) {
	err := service.RestoreNodeVestings(c.Request.Context(), config.GetDB(), in.NodeAddress, time.Now().UTC())
	if err != nil {
		if errors.Is(err, service.ErrInvalidVestingAddress) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &RestoreVestingResponse{}, nil
}
