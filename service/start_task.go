package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func getDefaultAbortIssuer() string {
	appConfig := config.GetConfig()
	for _, blockchain := range appConfig.Blockchains {
		return blockchain.Account.Address
	}
	return ""
}

type TaskTimeoutPhase string

const (
	TaskTimeoutPhaseQueue         TaskTimeoutPhase = "queue"
	TaskTimeoutPhaseExecution     TaskTimeoutPhase = "execution"
	TaskTimeoutPhaseAppValidation TaskTimeoutPhase = "app_validation"
	TaskTimeoutPhaseResultUpload  TaskTimeoutPhase = "result_upload"
	TaskTimeoutPhaseSDFT          TaskTimeoutPhase = "sdft"
)

func UsesRelayOwnedTimeouts(task *models.InferenceTask) bool {
	return task.TaskType == models.TaskTypeSD || task.TaskType == models.TaskTypeLLM
}

// GetQueueDeadline returns CreateTime plus the queue timeout for the task type.
// The result does not depend on the task's current status.
func GetQueueDeadline(task *models.InferenceTask) (time.Time, bool) {
	if !task.CreateTime.Valid {
		return time.Time{}, false
	}
	if task.TaskType == models.TaskTypeSDFTLora {
		return task.CreateTime.Time.Add(3*time.Minute + time.Duration(task.Timeout)*time.Second), true
	}
	if !UsesRelayOwnedTimeouts(task) {
		return time.Time{}, false
	}
	seconds := config.GetConfig().TaskPricing.QueueTimeoutSeconds
	return task.CreateTime.Time.Add(time.Duration(seconds) * time.Second), true
}

func GetTaskDeadline(task *models.InferenceTask) (time.Time, TaskTimeoutPhase, string, models.TaskAbortReason, bool) {
	pricing := config.GetConfig().TaskPricing
	if task.Status == models.TaskQueued {
		if deadline, ok := GetQueueDeadline(task); ok {
			if UsesRelayOwnedTimeouts(task) {
				return deadline, TaskTimeoutPhaseQueue, "relay", models.TaskAbortTimeout, true
			}
			return deadline, TaskTimeoutPhaseSDFT, "relay", models.TaskAbortTimeout, true
		}
		return time.Time{}, "", "", models.TaskAbortReasonNone, false
	}
	if !UsesRelayOwnedTimeouts(task) {
		if task.TaskType != models.TaskTypeSDFTLora {
			return time.Time{}, "", "", models.TaskAbortReasonNone, false
		}
		switch task.Status {
		case models.TaskStarted, models.TaskParametersUploaded, models.TaskErrorReported,
			models.TaskScoreReady, models.TaskValidated, models.TaskGroupValidated:
			if task.StartTime.Valid {
				return task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second),
					TaskTimeoutPhaseSDFT, "node_or_creator", models.TaskAbortTimeout, true
			}
		}
		return time.Time{}, "", "", models.TaskAbortReasonNone, false
	}

	switch task.Status {
	case models.TaskStarted, models.TaskParametersUploaded:
		if task.StartTime.Valid {
			return task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second),
				TaskTimeoutPhaseExecution, "node", models.TaskAbortTimeout, true
		}
	case models.TaskScoreReady, models.TaskErrorReported:
		if task.ScoreReadyTime.Valid {
			return task.ScoreReadyTime.Time.Add(time.Duration(pricing.AppValidationTimeoutSeconds) * time.Second),
				TaskTimeoutPhaseAppValidation, "app", models.TaskAbortCreatorValidationTimeout, true
		}
	case models.TaskValidated, models.TaskGroupValidated:
		if task.ValidatedTime.Valid {
			return task.ValidatedTime.Time.Add(time.Duration(pricing.ResultUploadTimeoutSeconds) * time.Second),
				TaskTimeoutPhaseResultUpload, "node", models.TaskAbortResultUploadTimeout, true
		}
	}
	return time.Time{}, "", "", models.TaskAbortReasonNone, false
}

func getQueuedTaskDeadline(task *models.InferenceTask) time.Time {
	deadline, _, _, _, _ := GetTaskDeadline(task)
	return deadline
}

func isQueuedTaskTimedOut(task *models.InferenceTask, now time.Time) bool {
	deadline, _, _, _, ok := GetTaskDeadline(task)
	return task.Status == models.TaskQueued && ok && !deadline.After(now)
}

func getRunningTaskDeadline(task *models.InferenceTask) time.Time {
	deadline, _, _, _, _ := GetTaskDeadline(task)
	return deadline
}

func isRunningTaskTimedOut(task *models.InferenceTask, now time.Time) bool {
	deadline, _, _, _, ok := GetTaskDeadline(task)
	return task.Status != models.TaskQueued && ok && !deadline.After(now)
}

func getTimedOutQueuedTasks(ctx context.Context, db *gorm.DB, now time.Time) ([]*models.InferenceTask, error) {
	const pageSize = 100
	tasks := make([]*models.InferenceTask, 0)
	queueTimeoutSeconds := config.GetConfig().TaskPricing.QueueTimeoutSeconds
	relayOwnedCutoff := now.Add(-time.Duration(queueTimeoutSeconds) * time.Second)
	// SDFT deadlines are CreateTime + 3 minutes + Timeout. Timeout is at least
	// 0, so CreateTime older than 3 minutes is the earliest possible candidate.
	sdftEarliestCutoff := now.Add(-3 * time.Minute)

	for offset := 0; ; offset += pageSize {
		page := make([]*models.InferenceTask, 0)
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		relayOwnedCandidate := db.Where(
			"task_type IN ?",
			[]models.TaskType{models.TaskTypeSD, models.TaskTypeLLM},
		).Where("create_time <= ?", relayOwnedCutoff)
		sdftCandidate := db.Where("task_type = ?", models.TaskTypeSDFTLora).
			Where("create_time <= ?", sdftEarliestCutoff)
		err := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
			Where("status = ?", models.TaskQueued).
			Where("create_time IS NOT NULL").
			Where(db.Where(relayOwnedCandidate).Or(sdftCandidate)).
			Order("id").
			Offset(offset).
			Limit(pageSize).
			Find(&page).Error
		cancel()
		if err != nil {
			return nil, err
		}

		for _, task := range page {
			if isQueuedTaskTimedOut(task, now) {
				tasks = append(tasks, task)
			}
		}
		if len(page) < pageSize {
			break
		}
	}

	return tasks, nil
}

func getTimedOutRunningTasks(ctx context.Context, db *gorm.DB, now time.Time) ([]*models.InferenceTask, error) {
	const pageSize = 100
	statuses := []models.TaskStatus{
		models.TaskStarted,
		models.TaskParametersUploaded,
		models.TaskErrorReported,
		models.TaskScoreReady,
		models.TaskValidated,
		models.TaskGroupValidated,
	}
	tasks := make([]*models.InferenceTask, 0)

	for offset := 0; ; offset += pageSize {
		page := make([]*models.InferenceTask, 0)
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
			Where("status IN ?", statuses).
			Where("start_time IS NOT NULL").
			Order("id").
			Offset(offset).
			Limit(pageSize).
			Find(&page).Error
		cancel()
		if err != nil {
			return nil, err
		}

		for _, task := range page {
			if isRunningTaskTimedOut(task, now) {
				tasks = append(tasks, task)
			}
		}
		if len(page) < pageSize {
			break
		}
	}

	return tasks, nil
}

func abortTimedOutTask(ctx context.Context, task *models.InferenceTask, abortIssuer string) error {
	var err error
	for range 3 {
		_, _, _, abortReason, ok := GetTaskDeadline(task)
		if !ok || !task.DeadlineAt.Valid || task.DeadlineAt.Time.After(time.Now()) {
			return nil
		}
		task.AbortReason = abortReason
		err = ExecuteNodeStateUpdate(ctx, config.GetDB(), []string{task.SelectedNode}, func() error {
			ctx1, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return SetTaskStatusEndAborted(ctx1, config.GetDB(), task, abortIssuer)
		})
		if err == nil {
			return nil
		}
		if errors.Is(err, models.ErrTaskStatusChanged) || errors.Is(err, models.ErrNodeStatusChanged) {
			currentTask, syncErr := models.GetTaskByIDCommitment(ctx, config.GetDB(), task.TaskIDCommitment)
			if syncErr != nil {
				return syncErr
			}
			*task = *currentTask
			continue
		}
		return err
	}
	return err
}

func abortTimedOutTasks(ctx context.Context, tasks []*models.InferenceTask, abortIssuer string) {
	for _, task := range tasks {
		log.Debugf("StartTask: task %s timeout, abort", task.TaskIDCommitment)
		if err := abortTimedOutTask(ctx, task, abortIssuer); err != nil {
			if errors.Is(err, errWrongTaskStatus) || errors.Is(err, models.ErrTaskStatusChanged) {
				log.Debugf("StartTask: abort timed out task %s skipped because status changed", task.TaskIDCommitment)
			} else if errors.Is(err, models.ErrNodeStatusChanged) {
				log.Debugf("StartTask: abort timed out task %s skipped because node status changed", task.TaskIDCommitment)
			} else {
				log.Errorf("StartTask: abort timed out task %s error: %v", task.TaskIDCommitment, err)
			}
		}
	}
}

type taskDeadlineCursor struct {
	Status     models.TaskStatus
	DeadlineAt time.Time
	ID         uint
}

func getDueTaskDeadlines(ctx context.Context, db *gorm.DB, now time.Time, limit int, cursor *taskDeadlineCursor) ([]*models.InferenceTask, error) {
	statuses := []models.TaskStatus{
		models.TaskQueued,
		models.TaskStarted,
		models.TaskParametersUploaded,
		models.TaskErrorReported,
		models.TaskScoreReady,
		models.TaskValidated,
		models.TaskGroupValidated,
	}
	var tasks []*models.InferenceTask
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	query := db.WithContext(dbCtx).
		Where("status IN ?", statuses).
		Where("deadline_at IS NOT NULL AND deadline_at <= ?", now)
	if cursor != nil {
		query = query.Where(
			"status > ? OR (status = ? AND deadline_at > ?) OR (status = ? AND deadline_at = ? AND id > ?)",
			cursor.Status,
			cursor.Status, cursor.DeadlineAt,
			cursor.Status, cursor.DeadlineAt, cursor.ID,
		)
	}
	err := query.
		Order("status ASC, deadline_at ASC, id ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func processTaskTimeouts(ctx context.Context) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	var cursor *taskDeadlineCursor

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			now := time.Now()
			abortIssuer := getDefaultAbortIssuer()
			if abortIssuer == "" {
				log.Debug("StartTask: skip timeout scan because abort issuer is not configured")
				timer.Reset(2 * time.Second)
				continue
			}

			tasks, err := getDueTaskDeadlines(ctx, config.GetDB(), now, config.GetConfig().Task.TimeoutQueryBatchSize, cursor)
			if err != nil {
				log.Errorf("StartTask: get due task deadlines error: %v", err)
			} else {
				if len(tasks) == 0 {
					cursor = nil
				} else {
					last := tasks[len(tasks)-1]
					cursor = &taskDeadlineCursor{
						Status:     last.Status,
						DeadlineAt: last.DeadlineAt.Time,
						ID:         last.ID,
					}
				}
				abortTimedOutTasks(ctx, tasks, abortIssuer)
			}

			timer.Reset(2 * time.Second)
		}
	}
}

func StartTaskProcesser(ctx context.Context) {
	go GetTaskTraceStore().StartCleanup(ctx)
	go processTaskTimeouts(ctx)
	go runMatchingScheduler(ctx)
}
