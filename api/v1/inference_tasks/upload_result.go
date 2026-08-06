package inference_tasks

import (
	"bytes"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ResultInput struct {
	TaskIDCommitment string `path:"task_id_commitment" json:"task_id_commitment" description:"Task id commitment" validate:"required"`
}

type ResultInputWithSignature struct {
	ResultInput
	Timestamp int64  `form:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `form:"signature" description:"Signature" validate:"required"`
}

func readLLMCompletionTokens(file *multipart.FileHeader) (uint64, error) {
	fileObj, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer fileObj.Close()
	var result struct {
		Usage *struct {
			CompletionTokens json.RawMessage `json:"completion_tokens"`
		} `json:"usage"`
	}
	decoder := json.NewDecoder(fileObj)
	if err := decoder.Decode(&result); err != nil {
		return 0, err
	}
	if result.Usage == nil || len(result.Usage.CompletionTokens) == 0 {
		return 0, errors.New("usage.completion_tokens is missing")
	}
	raw := bytes.TrimSpace(result.Usage.CompletionTokens)
	if len(raw) == 0 || raw[0] < '0' || raw[0] > '9' {
		return 0, errors.New("usage.completion_tokens must be a non-negative integer")
	}
	completionTokens, err := strconv.ParseUint(string(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("usage.completion_tokens must be a non-negative integer: %w", err)
	}
	return completionTokens, nil
}

func UploadResult(c *gin.Context, in *ResultInputWithSignature) (*response.Response, error) {

	match, address, err := validate.ValidateSignature(in.ResultInput, in.Timestamp, in.Signature)

	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	if !match {
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

	if task.SelectedNode != address {
		return nil, response.NewValidationErrorResponse("Signature", "Signer not allowed")
	}

	if task.Status != models.TaskValidated && task.Status != models.TaskGroupValidated && task.Status != models.TaskEndInvalidated && task.Status != models.TaskEndGroupRefund {
		validationErr := response.NewValidationErrorResponse("task_id_commitment", "Task not validated")
		return nil, validationErr
	}
	// Check whether the images are correct
	var uploadedScoreBytes []byte

	form, err := c.MultipartForm()
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	files, ok := form.File["files"]
	if !ok {
		return nil, response.NewValidationErrorResponse("files", "Files is empty")
	}

	for _, file := range files {

		fileObj, err := file.Open()

		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}

		var hash []byte
		if task.TaskType == models.TaskTypeSD || task.TaskType == models.TaskTypeSDFTLora {
			hash, err = blockchain.GetPHashForImage(fileObj)
		} else {
			hash, err = blockchain.GetHashForGPTResponse(fileObj)
		}

		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}

		uploadedScoreBytes = append(uploadedScoreBytes, hash...)

		err = fileObj.Close()
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	uploadedScore := hexutil.Encode(uploadedScoreBytes)

	log.Debugln("image compare: submitted score: " + task.Score)
	log.Debugln("image compare: score from the uploaded file: " + uploadedScore)

	if task.Score != uploadedScore {
		validationErr := response.NewValidationErrorResponse("files", "Wrong result files uploaded")
		return nil, validationErr
	}

	var completionTokens uint64
	var completionTokensValid bool
	if task.TaskType == models.TaskTypeLLM {
		if len(files) != 1 {
			log.Errorf("UploadResult: skip LLM calibration because result file count is invalid, task: %s, file_count: %d", task.TaskIDCommitment, len(files))
		} else if tokens, parseErr := readLLMCompletionTokens(files[0]); parseErr != nil {
			log.Errorf("UploadResult: skip LLM calibration because usage is invalid, task: %s, error: %v", task.TaskIDCommitment, parseErr)
		} else {
			completionTokens = tokens
			completionTokensValid = true
		}
	}

	taskGroup, err := models.GetTaskGroupByTaskID(c.Request.Context(), config.GetDB(), task.TaskID)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	isSlashed := task.Status == models.TaskEndInvalidated
	if !isSlashed && len(taskGroup) > 1 {
		for _, t := range taskGroup {
			if t.Status == models.TaskEndInvalidated {
				isSlashed = true
				break
			}
		}
	}

	appConfig := config.GetConfig()

	taskDir := filepath.Join(appConfig.DataDir.InferenceTasks, task.TaskIDCommitment, "results")
	if task.Status == models.TaskValidated || task.Status == models.TaskGroupValidated {
		if err = os.MkdirAll(taskDir, 0o711); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}
	slashedTaskDir := filepath.Join(appConfig.DataDir.SlashedTasks, task.TaskIDCommitment, "results")
	if isSlashed {
		if err = os.MkdirAll(slashedTaskDir, 0o711); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	var fileExt string
	if task.TaskType == models.TaskTypeSD || task.TaskType == models.TaskTypeSDFTLora {
		fileExt = ".png"
	} else {
		fileExt = ".json"
	}
	traceFiles := make([]service.TaskTraceUploadFile, 0, len(files))
	for i := range files {
		traceFiles = append(traceFiles, service.TaskTraceUploadFile{
			Index: strconv.Itoa(i),
			Type:  fileExt,
		})
	}
	checkpointPresent := false
	if task.TaskType == models.TaskTypeSDFTLora {
		checkpoints := form.File["checkpoint"]
		checkpointPresent = len(checkpoints) > 0
	}
	service.GetTaskTraceStore().RecordResultUploadStarted(task, traceFiles, checkpointPresent)

	for i, file := range files {
		if task.Status == models.TaskValidated || task.Status == models.TaskGroupValidated {
			filename := filepath.Join(taskDir, strconv.Itoa(i)+fileExt)
			if err := c.SaveUploadedFile(file, filename); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		}
		if isSlashed {
			slashedFilename := filepath.Join(slashedTaskDir, strconv.Itoa(i)+fileExt)
			if err := c.SaveUploadedFile(file, slashedFilename); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		}
	}

	// store checkpoint of finetune type task
	if task.TaskType == models.TaskTypeSDFTLora {
		var checkpoint *multipart.FileHeader
		if checkpoints, ok := form.File["checkpoint"]; !ok {
			return nil, response.NewValidationErrorResponse("checkpoint", "Checkpoint not uploaded")
		} else {
			if len(checkpoints) != 1 {
				return nil, response.NewValidationErrorResponse("checkpoint", "More than one checkpoint file")
			}
			checkpoint = checkpoints[0]
		}
		if task.Status == models.TaskValidated || task.Status == models.TaskGroupValidated {
			checkpointFilename := filepath.Join(taskDir, "checkpoint.zip")
			if err := c.SaveUploadedFile(checkpoint, checkpointFilename); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		}
		if isSlashed {
			slashedCheckpointFilename := filepath.Join(slashedTaskDir, "checkpoint.zip")
			if err := c.SaveUploadedFile(checkpoint, slashedCheckpointFilename); err != nil {
				return nil, response.NewExceptionResponse(err)
			}
		}
	}
	if isSlashed {
		if err := service.UpdatePendingSlashResultEvidence(c.Request.Context(), config.GetDB(), task.TaskIDCommitment); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}
	if task.Status == models.TaskValidated || task.Status == models.TaskGroupValidated {
		for range 3 {
			err = service.ExecuteNodeStateUpdate(c.Request.Context(), config.GetDB(), []string{task.SelectedNode}, func() error {
				return service.SetTaskStatusEndSuccess(c.Request.Context(), config.GetDB(), task)
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
		if task.TaskType == models.TaskTypeLLM {
			calibrateLLMGroupAfterUploadSuccess(c, task, completionTokens, completionTokensValid)
		}
	}
	return &response.Response{}, nil
}

// selectLLMGroupRefundCalibrationTasks returns same-score TaskEndGroupRefund
// members that share calibration with a TaskEndGroupSuccess upload.
func selectLLMGroupRefundCalibrationTasks(uploaded *models.InferenceTask, group []models.InferenceTask) []*models.InferenceTask {
	if uploaded.Status != models.TaskEndGroupSuccess {
		return nil
	}
	selected := make([]*models.InferenceTask, 0)
	for i := range group {
		groupTask := &group[i]
		if groupTask.Status == models.TaskEndGroupRefund && groupTask.Score == uploaded.Score {
			selected = append(selected, groupTask)
		}
	}
	return selected
}

// calibrateLLMGroupAfterUploadSuccess updates LLM execution parameters after a
// successful result upload. It MUST re-query the group after SetTaskStatusEndSuccess
// commits and MUST NOT reuse the taskGroup loaded before that terminal update,
// because that earlier view still reflects pre-success member statuses.
//
// If the post-success group re-query fails, this function deletes only the
// uploaded task snapshot and skips refund calibration. It MUST NOT delete
// TaskEndGroupRefund snapshots from the earlier group view; those snapshots
// remain until background cleanup removes them after the maximum remaining
// lifecycle.
func calibrateLLMGroupAfterUploadSuccess(c *gin.Context, task *models.InferenceTask, completionTokens uint64, completionTokensValid bool) {
	refreshedGroup, err := models.GetTaskGroupByTaskID(c.Request.Context(), config.GetDB(), task.TaskID)
	if err != nil {
		log.Errorf("UploadResult: skip LLM group refund calibration after successful upload because group re-query failed, task: %s, error: %v", task.TaskIDCommitment, err)
		service.DeleteTaskExecutionGPUSnapshot(task.TaskIDCommitment)
		return
	}

	if completionTokensValid {
		if err := service.CalibrateUploadedLLMTask(task, completionTokens); err != nil {
			log.Errorf("UploadResult: failed to calibrate uploaded LLM task, task: %s, error: %v", task.TaskIDCommitment, err)
		}
		for _, groupTask := range selectLLMGroupRefundCalibrationTasks(task, refreshedGroup) {
			if err := service.CalibrateUploadedLLMTask(groupTask, completionTokens); err != nil {
				log.Errorf("UploadResult: failed to calibrate group-refund LLM task, task: %s, error: %v", groupTask.TaskIDCommitment, err)
			}
		}
	}
	service.DeleteTaskExecutionGPUSnapshot(task.TaskIDCommitment)
	for i := range refreshedGroup {
		if refreshedGroup[i].Status == models.TaskEndGroupRefund {
			service.DeleteTaskExecutionGPUSnapshot(refreshedGroup[i].TaskIDCommitment)
		}
	}
}
