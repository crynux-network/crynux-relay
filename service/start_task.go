package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
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

func getQueuedTaskDeadline(task *models.InferenceTask) time.Time {
	return task.CreateTime.Time.Add(3*time.Minute + time.Duration(task.Timeout)*time.Second)
}

func isQueuedTaskTimedOut(task *models.InferenceTask, now time.Time) bool {
	return task.CreateTime.Valid && !getQueuedTaskDeadline(task).After(now)
}

func getRunningTaskDeadline(task *models.InferenceTask) time.Time {
	return task.StartTime.Time.Add(time.Duration(task.Timeout) * time.Second)
}

func isRunningTaskTimedOut(task *models.InferenceTask, now time.Time) bool {
	return task.StartTime.Valid && !getRunningTaskDeadline(task).After(now)
}

func getTimedOutQueuedTasks(ctx context.Context, db *gorm.DB, now time.Time) ([]*models.InferenceTask, error) {
	const pageSize = 100
	tasks := make([]*models.InferenceTask, 0)

	for offset := 0; ; offset += pageSize {
		page := make([]*models.InferenceTask, 0)
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
			Where("status = ?", models.TaskQueued).
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
	task.AbortReason = models.TaskAbortTimeout
	task.ValidatedTime = sql.NullTime{Time: time.Now(), Valid: true}

	var err error
	for range 3 {
		err = ExecuteNodeStateUpdate(ctx, config.GetDB(), []string{task.SelectedNode}, func() error {
			ctx1, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return SetTaskStatusEndAborted(ctx1, config.GetDB(), task, abortIssuer)
		})
		if err == nil {
			return nil
		}
		if errors.Is(err, models.ErrTaskStatusChanged) || errors.Is(err, models.ErrNodeStatusChanged) {
			if syncErr := task.SyncStatus(ctx, config.GetDB()); syncErr != nil {
				return syncErr
			}
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

func processTaskTimeouts(ctx context.Context) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

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

			queuedTasks, err := getTimedOutQueuedTasks(ctx, config.GetDB(), now)
			if err != nil {
				log.Errorf("StartTask: get timed out queued tasks error: %v", err)
			} else {
				abortTimedOutTasks(ctx, queuedTasks, abortIssuer)
			}

			runningTasks, err := getTimedOutRunningTasks(ctx, config.GetDB(), now)
			if err != nil {
				log.Errorf("StartTask: get timed out running tasks error: %v", err)
			} else {
				abortTimedOutTasks(ctx, runningTasks, abortIssuer)
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
