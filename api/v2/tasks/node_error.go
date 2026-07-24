package tasks

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NodeTaskErrorSigningInput struct {
	NodeAddress      string `json:"node_address" description:"Node address" validate:"required"`
	TaskIDCommitment string `json:"task_id_commitment" description:"Task id commitment" validate:"required"`
	TaskArgs         string `json:"task_args" description:"Task arguments used for execution" validate:"required"`
	ErrorType        string `json:"error_type" description:"Diagnostic error type" validate:"required"`
	Message          string `json:"message" description:"Diagnostic message" validate:"required"`
	StackTrace       string `json:"stack_trace" description:"Stack trace or no-traceback explanation" validate:"required"`
}

type NodeTaskErrorInput struct {
	NodeTaskErrorSigningInput
	PathTaskIDCommitment string `path:"task_id_commitment" json:"-" description:"Task id commitment path" validate:"required"`
	CapturedAt           int64  `json:"captured_at" description:"Node capture time" validate:"required"`
}

type NodeTaskErrorInputWithSignature struct {
	NodeTaskErrorInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func ReportNodeTaskError(c *gin.Context, in *NodeTaskErrorInputWithSignature) (*response.Response, error) {
	return reportNodeTaskError(c.Request.Context(), config.GetDB(), c.Param("task_id_commitment"), in)
}

func reportNodeTaskError(ctx context.Context, db *gorm.DB, pathTaskIDCommitment string, in *NodeTaskErrorInputWithSignature) (*response.Response, error) {
	if pathTaskIDCommitment != in.TaskIDCommitment {
		return nil, response.NewValidationErrorResponse("task_id_commitment", "Path and signed body do not match")
	}

	match, signerAddress, err := validate.ValidateSignature(in.NodeTaskErrorSigningInput, in.Timestamp, in.Signature)
	if err != nil || !match {
		if err != nil {
			log.WithError(err).Debug("node task error signature validation failed")
		}
		return nil, response.NewValidationErrorResponse("signature", "Invalid signature")
	}
	if !common.IsHexAddress(in.NodeAddress) || common.HexToAddress(in.NodeAddress) != common.HexToAddress(signerAddress) {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	task, err := models.GetTaskByIDCommitment(ctx, db, in.TaskIDCommitment)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}
	if !common.IsHexAddress(task.SelectedNode) || common.HexToAddress(task.SelectedNode) != common.HexToAddress(signerAddress) {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	record := models.NodeTaskError{
		NodeAddress:      common.HexToAddress(in.NodeAddress).Hex(),
		TaskIDCommitment: in.TaskIDCommitment,
		TaskArgs:         in.TaskArgs,
		ErrorType:        in.ErrorType,
		Message:          in.Message,
		StackTrace:       in.StackTrace,
		CapturedAt:       in.CapturedAt,
	}
	created, err := service.CreateNodeTaskError(ctx, db, &record)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	log.WithFields(log.Fields{
		"node_address":       record.NodeAddress,
		"task_id_commitment": record.TaskIDCommitment,
		"created":            created,
	}).Info("node task error report accepted")
	return &response.Response{}, nil
}
