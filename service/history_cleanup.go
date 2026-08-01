package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"crynux_relay/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var errHistoryCleanupBatchSize = errors.New("history cleanup batch size must be positive")

var terminalTaskStatuses = []models.TaskStatus{
	models.TaskEndInvalidated,
	models.TaskEndSuccess,
	models.TaskEndAborted,
	models.TaskEndGroupRefund,
	models.TaskEndGroupSuccess,
}

var pendingTaskLedgerEventTypes = []models.RelayAccountEventType{
	models.RelayAccountEventTypeTaskPayment,
	models.RelayAccountEventTypeTaskIncome,
	models.RelayAccountEventTypeDaoTaskShare,
	models.RelayAccountEventTypeTaskRefund,
	models.RelayAccountEventTypeUserDelegation,
}

const historyCleanupDBTimeout = 60 * time.Second

// CleanupTaskHistory deletes terminal inference tasks older than cutoff, their
// node_task_errors rows, and best-effort on-disk artifact directories. It does not
// delete events rows. Each select only loads id and task_id_commitment.
func CleanupTaskHistory(ctx context.Context, db *gorm.DB, cutoff time.Time, inferenceTasksDir, slashedTasksDir string, batchSize int) error {
	if batchSize <= 0 {
		return errHistoryCleanupBatchSize
	}

	var lastID uint
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		tasks, err := selectExpiredTerminalTasks(ctx, db, cutoff, lastID, batchSize)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}
		lastID = tasks[len(tasks)-1].ID

		blocked, err := blockedTaskCommitments(ctx, db, taskCommitments(tasks))
		if err != nil {
			return err
		}

		eligibleIDs := make([]uint, 0, len(tasks))
		eligibleCommitments := make([]string, 0, len(tasks))
		skipped := 0
		for _, task := range tasks {
			if _, skip := blocked[task.TaskIDCommitment]; skip {
				skipped++
				continue
			}
			eligibleIDs = append(eligibleIDs, task.ID)
			eligibleCommitments = append(eligibleCommitments, task.TaskIDCommitment)
		}
		if skipped > 0 {
			log.Infof("task history cleanup: skipped %d blocked terminal tasks", skipped)
		}

		if len(eligibleIDs) > 0 {
			dbCtx, cancel := context.WithTimeout(ctx, historyCleanupDBTimeout)
			err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("task_id_commitment IN ?", eligibleCommitments).
					Delete(&models.NodeTaskError{}).Error; err != nil {
					return err
				}
				if err := tx.Unscoped().Where("id IN ?", eligibleIDs).
					Delete(&models.InferenceTask{}).Error; err != nil {
					return err
				}
				return nil
			})
			cancel()
			if err != nil {
				return err
			}

			removeTaskArtifactDirs(inferenceTasksDir, slashedTasksDir, eligibleCommitments)
			log.Infof("task history cleanup: deleted %d terminal tasks older than %s", len(eligibleIDs), cutoff.UTC().Format(time.RFC3339))
		}

		if len(tasks) < batchSize {
			return nil
		}
	}
}

// CleanupEventHistory hard-deletes all events rows with created_at older than cutoff
// using batched DELETE ... LIMIT without loading full event rows into the app.
func CleanupEventHistory(ctx context.Context, db *gorm.DB, cutoff time.Time, batchSize int) error {
	if batchSize <= 0 {
		return errHistoryCleanupBatchSize
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		dbCtx, cancel := context.WithTimeout(ctx, historyCleanupDBTimeout)
		result := db.WithContext(dbCtx).
			Unscoped().
			Where("created_at < ?", cutoff).
			Order("id ASC").
			Limit(batchSize).
			Delete(&models.Event{})
		cancel()
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		log.Infof("event history cleanup: deleted %d events older than %s", result.RowsAffected, cutoff.UTC().Format(time.RFC3339))

		if result.RowsAffected < int64(batchSize) {
			return nil
		}
	}
}

type historyCleanupTask struct {
	ID               uint
	TaskIDCommitment string
}

func selectExpiredTerminalTasks(ctx context.Context, db *gorm.DB, cutoff time.Time, lastID uint, batchSize int) ([]historyCleanupTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, historyCleanupDBTimeout)
	defer cancel()

	var tasks []historyCleanupTask
	err := db.WithContext(dbCtx).
		Model(&models.InferenceTask{}).
		Select("id, task_id_commitment").
		Where("status IN ?", terminalTaskStatuses).
		Where("updated_at < ?", cutoff).
		Where("id > ?", lastID).
		Order("id ASC").
		Limit(batchSize).
		Find(&tasks).Error
	return tasks, err
}

func taskCommitments(tasks []historyCleanupTask) []string {
	commitments := make([]string, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.TaskIDCommitment == "" {
			continue
		}
		if _, ok := seen[task.TaskIDCommitment]; ok {
			continue
		}
		seen[task.TaskIDCommitment] = struct{}{}
		commitments = append(commitments, task.TaskIDCommitment)
	}
	return commitments
}

func blockedTaskCommitments(ctx context.Context, db *gorm.DB, commitments []string) (map[string]struct{}, error) {
	blocked := make(map[string]struct{})
	if len(commitments) == 0 {
		return blocked, nil
	}

	commitmentSet := make(map[string]struct{}, len(commitments))
	for _, commitment := range commitments {
		commitmentSet[commitment] = struct{}{}
	}

	dbCtx, cancel := context.WithTimeout(ctx, historyCleanupDBTimeout)
	defer cancel()

	var pendingSlashCommitments []string
	if err := db.WithContext(dbCtx).
		Model(&models.PendingSlash{}).
		Where("status = ?", models.PendingSlashStatusPending).
		Where("task_id_commitment IN ?", commitments).
		Pluck("task_id_commitment", &pendingSlashCommitments).Error; err != nil {
		return nil, err
	}
	for _, commitment := range pendingSlashCommitments {
		blocked[commitment] = struct{}{}
	}

	var pendingEvents []models.RelayAccountEvent
	if err := db.WithContext(dbCtx).
		Model(&models.RelayAccountEvent{}).
		Select("type, reason").
		Where("status = ?", models.RelayAccountEventStatusPending).
		Where("type IN ?", pendingTaskLedgerEventTypes).
		Find(&pendingEvents).Error; err != nil {
		return nil, err
	}
	for _, event := range pendingEvents {
		reasons, ok := splitRelayAccountEventReason(event.Type, event.Reason)
		if !ok || len(reasons) < 2 {
			continue
		}
		commitment := reasons[1]
		if _, match := commitmentSet[commitment]; match {
			blocked[commitment] = struct{}{}
		}
	}

	return blocked, nil
}

func removeTaskArtifactDirs(inferenceTasksDir, slashedTasksDir string, commitments []string) {
	for _, commitment := range commitments {
		if commitment == "" {
			continue
		}
		if inferenceTasksDir != "" {
			path := filepath.Join(inferenceTasksDir, commitment)
			if err := os.RemoveAll(path); err != nil {
				log.Errorf("task history cleanup: failed to remove inference task dir %s: %v", path, err)
			}
		}
		if slashedTasksDir != "" {
			path := filepath.Join(slashedTasksDir, commitment)
			if err := os.RemoveAll(path); err != nil {
				log.Errorf("task history cleanup: failed to remove slashed task dir %s: %v", path, err)
			}
		}
	}
}
