package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"database/sql"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AbortTaskInput struct {
	TaskIDCommitment string                 `path:"task_id_commitment" json:"task_id_commitment" description:"Task id commitment" validate:"required"`
	AbortReason      models.TaskAbortReason `json:"abort_reason" description:"Task abort reason" validate:"required"`
}

type AbortTaskInputWithSignature struct {
	AbortTaskInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func AbortTask(c *gin.Context, in *AbortTaskInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.AbortTaskInput, in.Timestamp, in.Signature)

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

	if !(address == task.Creator || address == task.SelectedNode) {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	if task.StartTime.Valid && task.StartTime.Time.Add(time.Duration(task.Timeout)*time.Second).Compare(time.Now()) > 0 {
		return nil, response.NewValidationErrorResponse("task_id_commitment", "Timeout not reached")
	}

	task.AbortReason = in.AbortReason
	if !task.ValidatedTime.Valid {
		task.ValidatedTime = sql.NullTime{Time: time.Now(), Valid: true}
	}
	for range 3 {
		err = service.ExecuteNodeStateUpdate(c.Request.Context(), config.GetDB(), []string{task.SelectedNode}, func() error {
			return service.SetTaskStatusEndAborted(c.Request.Context(), config.GetDB(), task, address)
		})
		if err == nil {
			break
		} else if errors.Is(err, models.ErrTaskStatusChanged) || errors.Is(err, models.ErrNodeStatusChanged) {
			if err := task.SyncStatus(c.Request.Context(), config.GetDB()); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		} else {
			return nil, response.NewExceptionResponse(err)
		}
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
