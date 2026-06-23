package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
)

type KickoutAllNodesInput struct {
	Network     string `json:"network" validate:"required"`
	AbortIssuer string `json:"abort_issuer"`
}

type KickoutAllNodesResponse struct {
	response.Response
	Data service.AdminMigrationResetResult `json:"data"`
}

func KickoutAllNodes(c *gin.Context, in *KickoutAllNodesInput) (*KickoutAllNodesResponse, error) {
	result, err := service.AdminKickoutAllNodesAndAbortTasks(c.Request.Context(), config.GetDB(), in.Network, in.AbortIssuer)
	if err != nil {
		if errors.Is(err, service.ErrMigrationNetworkRequired) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &KickoutAllNodesResponse{Data: *result}, nil
}
