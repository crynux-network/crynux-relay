package inference_tasks

import (
	"archive/zip"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type GetResultInput struct {
	TaskIDCommitment string `path:"task_id_commitment" json:"task_id_commitment" description:"Task id commitment" validate:"required"`
}

type GetResultInputWithSignature struct {
	GetResultInput
	Timestamp int64  `query:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" description:"Signature" validate:"required"`
}

func GetResult(c *gin.Context, in *GetResultInputWithSignature) error {

	match, address, err := validate.ValidateSignature(in.GetResultInput, in.Timestamp, in.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln(err)
		}

		return response.NewValidationErrorResponse("signature", "Invalid signature")
	}

	task, err := models.GetTaskByIDCommitment(c.Request.Context(), config.GetDB(), in.TaskIDCommitment)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErr := response.NewValidationErrorResponse("task_id_commitment", "Task not found")
			return validationErr
		} else {
			return response.NewExceptionResponse(err)
		}
	}

	if task.Creator != address {
		return response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	if task.Status != models.TaskEndSuccess && task.Status != models.TaskEndGroupSuccess {
		return response.NewValidationErrorResponse("task_id", "Task results not uploaded")
	}

	if task.TaskType == models.TaskTypeSDFTLora {
		return response.NewValidationErrorResponse("task_id_commitment", "Whole-task result download is not available for fine-tune tasks")
	}
	if task.TaskSize == 0 {
		return response.NewValidationErrorResponse("task_id_commitment", "Task result size is invalid")
	}

	resultDir := filepath.Join(config.GetConfig().DataDir.InferenceTasks, task.TaskIDCommitment, "results")
	extension := ".png"
	if task.TaskType == models.TaskTypeLLM {
		if task.TaskSize != 1 {
			return response.NewValidationErrorResponse("task_id_commitment", "LLM task size must be 1")
		}
		extension = ".json"
	}
	files := make([]*os.File, 0, task.TaskSize)
	defer func() {
		for _, file := range files {
			_ = file.Close()
		}
	}()
	var totalSize int64
	for i := uint64(0); i < task.TaskSize; i++ {
		resultFile, err := validatedResultPath(resultDir, i, extension)
		if err != nil {
			return response.NewExceptionResponse(err)
		}
		file, err := os.Open(resultFile)
		if err != nil {
			return response.NewValidationErrorResponse("task_id_commitment", "Task result files are incomplete")
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = file.Close()
			return response.NewValidationErrorResponse("task_id_commitment", "Task result files are invalid")
		}
		totalSize += info.Size()
		if totalSize > config.GetConfig().Task.ResultMaxUncompressedBytes {
			_ = file.Close()
			return response.NewValidationErrorResponse("task_id_commitment", "Task result exceeds maximum aggregate size")
		}
		files = append(files, file)
	}

	service.GetTaskTraceStore().RecordAppResultFetched(task.TaskIDCommitment, "task_results", "")
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	if task.TaskType == models.TaskTypeLLM {
		c.Header("Content-Disposition", "attachment; filename=0.json")
		c.Header("Content-Type", "application/json")
		written, err := io.Copy(c.Writer, files[0])
		metrics.TaskResultDownloadBytes.WithLabelValues("llm").Add(float64(written))
		return err
	}

	c.Header("Content-Disposition", "attachment; filename=results.zip")
	c.Header("Content-Type", "application/zip")
	counter := &countingWriter{Writer: c.Writer}
	archive := zip.NewWriter(counter)
	for i, file := range files {
		entry, err := archive.Create(strconv.Itoa(i) + ".png")
		if err != nil {
			_ = archive.Close()
			return err
		}
		if _, err := io.Copy(entry, file); err != nil {
			_ = archive.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	metrics.TaskResultDownloadBytes.WithLabelValues("sd").Add(float64(counter.Count))
	return nil
}

type countingWriter struct {
	io.Writer
	Count int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.Count += int64(n)
	return n, err
}

func validatedResultPath(resultDir string, index uint64, extension string) (string, error) {
	base, err := filepath.Abs(resultDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, fmt.Sprintf("%d%s", index, extension))
	relative, err := filepath.Rel(base, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid result path")
	}
	return path, nil
}

type GetResultCheckpointInput struct {
	TaskIDCommitment string `path:"task_id_commitment" json:"task_id_commitment" description:"Task id commitment" validate:"required"`
}

type GetResultCheckpointInputWithSignature struct {
	GetResultCheckpointInput
	Timestamp int64  `query:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" description:"Signature" validate:"required"`
}

func GetResultCheckpoint(c *gin.Context, in *GetResultCheckpointInputWithSignature) error {
	match, address, err := validate.ValidateSignature(in.GetResultCheckpointInput, in.Timestamp, in.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln(err)
		}

		return response.NewValidationErrorResponse("signature", "Invalid signature")
	}

	task, err := models.GetTaskByIDCommitment(c.Request.Context(), config.GetDB(), in.TaskIDCommitment)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErr := response.NewValidationErrorResponse("task_id_commitment", "Task not found")
			return validationErr
		} else {
			return response.NewExceptionResponse(err)
		}
	}

	if task.Creator != address {
		return response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	if task.Status != models.TaskEndSuccess {
		return response.NewValidationErrorResponse("task_id", "Task checkpoint not uploaded")
	}

	appConfig := config.GetConfig()
	resultFile := filepath.Join(
		appConfig.DataDir.InferenceTasks,
		task.TaskIDCommitment,
		"results",
		"checkpoint.zip",
	)

	if _, err := os.Stat(resultFile); err != nil {
		return response.NewValidationErrorResponse("task_id", "Checkpoint file not found")
	}
	service.GetTaskTraceStore().RecordAppResultFetched(task.TaskIDCommitment, "checkpoint", "")

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename=checkpoint.zip")
	c.Header("Content-Type", "application/octet-stream")
	c.File(resultFile)

	return nil

}
