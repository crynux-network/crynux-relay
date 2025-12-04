package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type GetSelectedNodeInfoInput struct {
	TaskIDCommitment string `path:"task_id_commitment" json:"task_id_commitment" validate:"required" description:"The task id commitment"`
}

type GetSelectedNodeInfoInputWithSignature struct {
	GetSelectedNodeInfoInput
	Timestamp int64  `query:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" description:"Signature" validate:"required"`
}

type SelectedNodeInfo struct {
	Address string            `json:"address" gorm:"index"`
	Status  models.NodeStatus `json:"status" gorm:"index"`
	GPUName string            `json:"gpu_name" gorm:"index"`
	GPUVram uint64            `json:"gpu_vram" gorm:"index"`
	Version string            `json:"version"`
	Network string            `json:"network"`
}

type GetSelectedNodeInfoResponse struct {
	response.Response
	Data *SelectedNodeInfo
}

func GetSelectedNodeInfo(c *gin.Context, in *GetSelectedNodeInfoInputWithSignature) (*GetSelectedNodeInfoResponse, error) {
	match, address, err := validate.ValidateSignature(in.GetSelectedNodeInfoInput, in.Timestamp, in.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}

		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	task, err := models.GetTaskByIDCommitment(c.Request.Context(), config.GetDB(), in.TaskIDCommitment)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErr := response.NewValidationErrorResponse("task_id_commitment", "Task not found")
			return nil, validationErr
		} else {
			return nil, response.NewExceptionResponse(err)
		}
	}

	if len(task.SelectedNode) == 0 {
		return nil, response.NewValidationErrorResponse("task_id_commitment", "Task not started")
	}

	if task.SelectedNode != address && task.Creator != address {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), task.SelectedNode)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	nodeVersion := fmt.Sprintf("%d.%d.%d", node.MajorVersion, node.MinorVersion, node.PatchVersion)
	return &GetSelectedNodeInfoResponse{
		Data: &SelectedNodeInfo{
			Address: node.Address,
			Status:  node.Status,
			GPUName: node.GPUName,
			GPUVram: node.GPUVram,
			Version: nodeVersion,
			Network: node.Network,
		},
	}, nil
}
