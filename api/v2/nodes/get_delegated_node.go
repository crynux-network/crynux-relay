package nodes

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getDelegatedNodeSnapshot(ctx context.Context, db *gorm.DB, nodeAddress string) (*models.DelegatedStakingNodeListSnapshot, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var snapshot models.DelegatedStakingNodeListSnapshot
	if err := db.WithContext(dbCtx).Where("node_address = ?", nodeAddress).First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &snapshot, nil
}

func GetDelegatedNode(c *gin.Context, input *GetNodeInput) (*NodeResponse, error) {
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
	snapshot, err := getDelegatedNodeSnapshot(c.Request.Context(), config.GetDB(), node.Address)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	applyDelegationAPRSnapshot(nodeData, snapshot)

	return &NodeResponse{
		Data: nodeData,
	}, nil
}
