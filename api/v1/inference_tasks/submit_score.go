package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type SubmitScoreInput struct {
	TaskIDCommitment string `path:"task_id_commitment" json:"task_id_commitment" description:"Task id commitment" validate:"required"`
	Score            string `json:"score" description:"task score" vaidate:"required"`
}

type SubmitScoreInputWithSignature struct {
	SubmitScoreInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func SubmitScore(c *gin.Context, in *SubmitScoreInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.SubmitScoreInput, in.Timestamp, in.Signature)

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

	if task.Status != models.TaskStarted {
		return nil, response.NewValidationErrorResponse("task_id_commitment", "Illegal task state")
	}

	if task.SelectedNode != address {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	scoreBytes, err := hexutil.Decode(in.Score)
	if err != nil {
		return nil, response.NewValidationErrorResponse("score", "invalid score")
	}
	if (task.TaskType == models.TaskTypeSD || task.TaskType == models.TaskTypeSDFTLora) && len(scoreBytes)%8 != 0 {
		return nil, response.NewValidationErrorResponse("score", "invalid score")
	}

	task.Score = in.Score
	for range 3 {
		err = service.SetTaskStatusScoreReady(c.Request.Context(), config.GetDB(), task)
		if err == nil {
			break
		} else if errors.Is(err, models.ErrTaskStatusChanged) || errors.Is(err, models.ErrNodeStatusChanged) {
			if err := task.SyncStatus(c.Request.Context(), config.GetDB()); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		} else if errors.Is(err, service.ErrWrongNodeCurrentTask) {
			return nil, response.NewValidationErrorResponse("task_id_commitment", err.Error())
		} else {
			return nil, response.NewExceptionResponse(err)
		}
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
