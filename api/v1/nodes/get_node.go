package nodes

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetNodeInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
}

func GetNode(c *gin.Context, input *GetNodeInput) (*NodeResponse, error) {
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), input.Address)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &NodeResponse{
			Data: &Node{
				Address:            input.Address,
				Status:             models.NodeStatusQuit,
				GPUName:            "",
				GPUVram:            0,
				QOSScore:           0,
				Version:            "",
				ModelFiles:         []models.ModelFileEntry{},
				InMemoryModelFiles: []models.ModelFileEntry{},
			},
		}, nil
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	modelFiles, inMemoryModelFiles, err := service.NodeModelFileEntries(c.Request.Context(), config.GetDB(), node.Address)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeVersion := fmt.Sprintf("%d.%d.%d", node.MajorVersion, node.MinorVersion, node.PatchVersion)

	qosScore := uint64(node.QOSScore)

	return &NodeResponse{
		Data: &Node{
			Address:            node.Address,
			Status:             node.Status,
			GPUName:            node.GPUName,
			GPUVram:            node.GPUVram,
			QOSScore:           qosScore,
			Version:            nodeVersion,
			ModelFiles:         modelFiles,
			InMemoryModelFiles: inMemoryModelFiles,
		},
	}, nil
}
