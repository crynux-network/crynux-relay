package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"

	log "github.com/sirupsen/logrus"
)

func logTaskAssignmentEvent(ctx context.Context, task *models.InferenceTask, candidateCount int) {
	logger := config.GetTaskAssignmentLogger()
	if logger == nil {
		return
	}

	queueSizeLabel := "unknown"
	queueSize, err := models.GetQueuedTaskCount(ctx, config.GetDB())
	if err != nil {
		log.Errorf("TaskAssignment: get queued task count error: %v", err)
	} else {
		queueSizeLabel = fmt.Sprintf("%d", queueSize)
	}

	logger.Infof(
		"[TaskAssignment] [%s] [%s] [candidate_count=%d] [queue_size=%s] task_id=%s",
		taskTypeTag(task.TaskType),
		taskVramRequirementLabel(task),
		candidateCount,
		queueSizeLabel,
		task.TaskIDCommitment,
	)
}

func taskVramRequirementLabel(task *models.InferenceTask) string {
	if len(task.RequiredGPU) > 0 {
		return fmt.Sprintf("%dGB", task.RequiredGPUVRAM)
	}
	return fmt.Sprintf("%dGB", task.MinVRAM)
}
