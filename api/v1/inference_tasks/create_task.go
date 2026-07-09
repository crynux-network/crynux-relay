package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"crypto/rand"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type TaskInput struct {
	TaskIDCommitment string          `path:"task_id_commitment" json:"task_id_commitment" description:"Task id commitment" validate:"required"`
	TaskArgs         string          `form:"task_args" json:"task_args" description:"Task arguments" validate:"required"`
	TaskType         models.TaskType `form:"task_type" json:"task_type" description:"Task type"`
	Nonce            string          `form:"nonce" json:"nonce" description:"nonce" validate:"required"`
	TaskModelIDs     []string        `form:"task_model_ids" json:"task_model_ids" description:"task model ids" validate:"required"`
	MinVram          *uint64         `form:"min_vram" json:"min_vram" description:"min vram"`
	RequiredGPU      *string         `form:"required_gpu" json:"required_gpu" description:"required gpu name"`
	RequiredGPUVram  *uint64         `form:"required_gpu_vram" json:"required_gpu_vram" description:"required gpu vram"`
	TaskVersion      string          `form:"task_version" json:"task_version" description:"task version" validate:"required"`
	TaskSize         *uint64         `form:"task_size" json:"task_size" description:"task size"`
	TaskFee          models.BigInt   `form:"task_fee" json:"task_fee" description:"task fee, in unit wei" validate:"required"`
	Timeout          uint64          `form:"timeout" json:"timeout" description:"timeout, in minutes" validate:"required"`
}

type TaskInputWithSignature struct {
	TaskInput
	Timestamp int64  `form:"timestamp" json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `form:"signature" json:"signature" description:"Signature" validate:"required"`
}

func CreateTask(c *gin.Context, in *TaskInputWithSignature) (*TaskResponse, error) {
	match, address, err := validate.ValidateSignature(in.TaskInput, in.Timestamp, in.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}

		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	in.TaskModelIDs = models.NormalizeModelIDs(in.TaskModelIDs)

	if config.GetConfig().Task.TaskWhitelistEnabled {
		allowed, err := service.IsTaskCreatorWhitelisted(c.Request.Context(), config.GetDB(), address)
		if err != nil {
			if errors.Is(err, service.ErrInvalidTaskWhitelistAddress) {
				return nil, response.NewValidationErrorResponse("address", "Invalid address")
			}
			return nil, response.NewExceptionResponse(err)
		}
		if !allowed {
			return nil, response.NewValidationErrorResponse("address", "Signer not allowed")
		}
	}

	validationErr, err := models.ValidateTaskArgsJsonStr(in.TaskArgs, in.TaskType)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if validationErr != nil {
		return nil, response.NewValidationErrorResponse("task_args", validationErr.Error())
	}

	normalizedTaskArgs, err := models.NormalizeTaskArgsModelNames(in.TaskArgs, in.TaskType)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	in.TaskArgs = normalizedTaskArgs

	taskVersions := strings.Split(in.TaskVersion, ".")
	if len(taskVersions) != 3 {
		return nil, response.NewValidationErrorResponse("task_version", "Invalid task version")
	}
	for i := 0; i < 3; i++ {
		if _, err := strconv.ParseUint(taskVersions[i], 10, 64); err != nil {
			return nil, response.NewValidationErrorResponse("task_version", "Invalid task version")
		}
	}

	_, err = models.GetTaskByIDCommitment(c.Request.Context(), config.GetDB(), in.TaskIDCommitment)
	if err == nil {
		return nil, response.NewValidationErrorResponse("task_id_commitment", "Task already uploaded")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, response.NewExceptionResponse(err)
	}

	if in.TaskType == models.TaskTypeSDFTLora && c.ContentType() == "multipart/form-data" {
		form, err := c.MultipartForm()
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
		if files, ok := form.File["checkpoint"]; ok {
			if len(files) != 1 {
				return nil, response.NewValidationErrorResponse("checkpoint", "More than one checkpoint file uploaded")
			}
			checkpoint := files[0]

			appConfig := config.GetConfig()

			taskDir := filepath.Join(appConfig.DataDir.InferenceTasks, in.TaskIDCommitment, "input")
			if err = os.MkdirAll(taskDir, 0o711); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
			checkpointFilename := filepath.Join(taskDir, "checkpoint.zip")
			if err := c.SaveUploadedFile(checkpoint, checkpointFilename); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		}
	}

	samplingSeedBytes := make([]byte, 32)
	if _, err := rand.Read(samplingSeedBytes); err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	samplingSeed := hexutil.Encode(samplingSeedBytes)

	task := &models.InferenceTask{
		TaskArgs:         in.TaskArgs,
		TaskIDCommitment: in.TaskIDCommitment,
		Creator:          address,
		SamplingSeed:     samplingSeed,
		Nonce:            in.Nonce,
		Status:           models.TaskQueued,
		TaskType:         in.TaskType,
		TaskVersion:      in.TaskVersion,
		MinVRAM:          *in.MinVram,
		RequiredGPU:      *in.RequiredGPU,
		RequiredGPUVRAM:  *in.RequiredGPUVram,
		TaskFee:          in.TaskFee,
		TaskSize:         *in.TaskSize,
		ModelIDs:         in.TaskModelIDs,
		CreateTime: sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		},
		Timeout: in.Timeout,
	}

	if err := service.CreateTask(c.Request.Context(), config.GetDB(), task); err != nil {
		if errors.Is(err, service.ErrInsufficientRelayAccount) {
			return nil, response.NewValidationErrorResponse("task_fee", "Insufficient relay account balance")
		}
		return nil, response.NewExceptionResponse(err)
	}

	return &TaskResponse{Data: &InferenceTask{
		Sequence:         uint64(task.ID),
		TaskArgs:         task.TaskArgs,
		TaskIDCommitment: task.TaskIDCommitment,
		Creator:          task.Creator,
		SamplingSeed:     task.SamplingSeed,
		Nonce:            task.Nonce,
		Status:           task.Status,
		TaskType:         task.TaskType,
		TaskVersion:      task.TaskVersion,
		MinVRAM:          task.MinVRAM,
		RequiredGPU:      task.RequiredGPU,
		RequiredGPUVRAM:  task.RequiredGPUVRAM,
		TaskFee:          task.TaskFee,
		TaskSize:         task.TaskSize,
		ModelIDs:         task.ModelIDs,
		CreateTime:       &task.CreateTime.Time,
		Timeout:          task.Timeout,
	}}, nil
}
