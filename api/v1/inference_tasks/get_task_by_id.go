package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type GetTaskInput struct {
	TaskIDCommitment string `path:"task_id_commitment" json:"task_id_commitment" validate:"required" description:"The task id commitment"`
}

type GetTaskInputWithSignature struct {
	GetTaskInput
	Timestamp int64  `query:"timestamp" json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" json:"signature" description:"Signature" validate:"required"`
}

func GetTaskById(c *gin.Context, in *GetTaskInputWithSignature) (*TaskResponse, error) {

	match, address, err := validate.ValidateSignature(in.GetTaskInput, in.Timestamp, in.Signature)

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

	if len(task.TaskArgs) == 0 {
		return nil, response.NewValidationErrorResponse("task_id_commitment", "Task not ready")
	}

	if task.SelectedNode != address && task.Creator != address {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	if task.SelectedNode == address {
		if err := service.MarkTaskDelivered(c.Request.Context(), config.GetDB(), task); err != nil {
			log.Errorf("GetTaskById: failed to mark task delivered, task: %s, node: %s, error: %v", task.TaskIDCommitment, address, err)
		}
	}

	qosScore := uint64(0)
	if task.QOSScore.Valid {
		qosScore = uint64(task.QOSScore.Int64)
	}

	t := &InferenceTask{
		Sequence:         uint64(task.ID),
		TaskArgs:         task.TaskArgs,
		TaskIDCommitment: task.TaskIDCommitment,
		Creator:          task.Creator,
		SamplingSeed:     task.SamplingSeed,
		Nonce:            task.Nonce,
		Status:           task.Status,
		TaskType:         task.TaskType,
		TaskVersion:      task.TaskVersion,
		Timeout:          task.Timeout,
		MinVRAM:          task.MinVRAM,
		RequiredGPU:      task.RequiredGPU,
		RequiredGPUVRAM:  task.RequiredGPUVRAM,
		TaskFee:          task.TaskFee,
		TaskSize:         task.TaskSize,
		ModelIDs:         task.ModelIDs,
		AbortReason:      task.AbortReason,
		TaskError:        task.TaskError,
		Score:            task.Score,
		QOSScore:         qosScore,
		SelectedNode:     task.SelectedNode,
	}
	if task.CreateTime.Valid {
		t.CreateTime = &task.CreateTime.Time
	}
	if task.StartTime.Valid {
		t.StartTime = &task.StartTime.Time
	}
	if task.ScoreReadyTime.Valid {
		t.ScoreReadyTime = &task.ScoreReadyTime.Time
	}
	if task.ValidatedTime.Valid {
		t.ValidatedTime = &task.ValidatedTime.Time
	}
	if task.ResultUploadedTime.Valid {
		t.ResultUploadedTime = &task.ResultUploadedTime.Time
	}
	return &TaskResponse{Data: t}, nil
}
