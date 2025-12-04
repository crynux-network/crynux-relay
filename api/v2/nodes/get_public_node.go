package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
func GetPublicNode(c *gin.Context, input *GetNodeInput) (*NodeResponse, error) {
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), input.Address)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, response.NewNotFoundErrorResponse()
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	if node.DelegatorShare == 0 {
		return nil, response.NewNotFoundErrorResponse()
	}

	nodeData, err := getNodeData(c.Request.Context(), node)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &NodeResponse{
		Data: nodeData,
	}, nil
}
