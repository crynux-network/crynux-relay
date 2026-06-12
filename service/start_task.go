package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/rand"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DispatchedTask struct {
	task      *models.InferenceTask
	node      *models.Node
	resChan   chan bool
	createdAt time.Time
	finished  bool
	mu        sync.RWMutex
}

type TaskDispatcher struct {
	nodeQueue        chan string
	taskMap          sync.Map
	processingTasks  sync.Map
	dispatchLimiter  chan struct{}
	startTaskLimiter chan struct{}
}

func getDefaultAbortIssuer() string {
	appConfig := config.GetConfig()
	for _, blockchain := range appConfig.Blockchains {
		return blockchain.Account.Address
	}
	return ""
}

func NewTaskDispatcher() *TaskDispatcher {
	return &TaskDispatcher{
		nodeQueue:        make(chan string, 100),
		dispatchLimiter:  make(chan struct{}, 100),
		startTaskLimiter: make(chan struct{}, 100),
	}
}

func (d *TaskDispatcher) getQueuedTasks(ctx context.Context) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	limit := 100
	for {
		select {
		case <-ctx.Done():
			return
		default:
			tasks, err := func(ctx context.Context) ([]*models.InferenceTask, error) {
				tasks := make([]*models.InferenceTask, 0)

				dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				err := config.GetDB().WithContext(dbCtx).Model(&models.InferenceTask{}).
					Where("status = ?", models.TaskQueued).
					Order("id").
					Limit(limit).
					Find(&tasks).Error
				if err != nil {
					return nil, err
				}
				return tasks, nil
			}(ctx)
			if err == nil && len(tasks) > 0 {
				dispatchableTaskCount := 0
				now := time.Now()
				for _, task := range tasks {
					if isQueuedTaskTimedOut(task, now) {
						log.Debugf("StartTask: task %s timeout, skip dispatch", task.TaskIDCommitment)
						continue
					}
					if _, loaded := d.processingTasks.LoadOrStore(task.ID, struct{}{}); loaded {
						continue
					}
					dispatchableTaskCount++
					go func(task *models.InferenceTask) {
						d.dispatchLimiter <- struct{}{}
						defer func() {
							<-d.dispatchLimiter
							d.processingTasks.Delete(task.ID)
						}()

						if err := task.SyncStatus(ctx, config.GetDB()); err != nil {
							log.Errorf("StartTask: sync task status error: %v", err)
							return
						}
						if task.Status != models.TaskQueued {
							return
						}

						deadline := getQueuedTaskDeadline(task)
						if !deadline.After(time.Now()) {
							log.Debugf("StartTask: task %s timeout, skip dispatch", task.TaskIDCommitment)
							return
						}
						ctx1, cancel := context.WithDeadline(ctx, deadline)
						defer cancel()
						log.Debugf("StartTask: dispatch task %s", task.TaskIDCommitment)
						d.Dispatch(ctx1, task)
					}(task)

				}
				if dispatchableTaskCount > 0 {
					continue
				}
			}
			if err != nil {
				log.Errorf("StartTask: get queued tasks error: %v", err)

			}

			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(2 * time.Second)

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}
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
		ctx1, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = SetTaskStatusEndAborted(ctx1, config.GetDB(), task, abortIssuer)
		cancel()
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

func (d *TaskDispatcher) Process(ctx context.Context, task *models.InferenceTask, node *models.Node) bool {
	dispatchedTask, loaded := d.taskMap.LoadOrStore(node.Address, &DispatchedTask{
		task:      task,
		node:      node,
		resChan:   make(chan bool, 1),
		createdAt: time.Now(),
		finished:  false,
	})
	if !loaded {
		log.Debugf("StartTask: new dispatched task %s on node %s", task.TaskIDCommitment, node.Address)
		resChan := dispatchedTask.(*DispatchedTask).resChan
		d.nodeQueue <- node.Address
		log.Debugf("StartTask: waiting for task %s on node %s", task.TaskIDCommitment, node.Address)
		select {
		case res := <-resChan:
			return res
		case <-ctx.Done():
			return false
		}

	} else {
		dispatchedTask, _ := dispatchedTask.(*DispatchedTask)
		if dispatchedTask.mu.TryLock() {
			if dispatchedTask.finished {
				dispatchedTask.mu.Unlock()
				log.Debugf("StartTask: node %s has been dispatched a task, skip", node.Address)
				return false
			}
			originalTask := dispatchedTask.task
			if originalTask.TaskFee.Cmp(&task.TaskFee.Int) >= 0 {
				dispatchedTask.mu.Unlock()
				log.Debugf("StartTask: task %s fee is lower than original task fee, skip", task.TaskIDCommitment)
				return false
			}
			// if current task fee is higher than original task fee, replace the original task
			log.Debugf("StartTask: task %s fee is higher than original task fee, replace", task.TaskIDCommitment)
			log.Debugf("StartTask: task %s is replaced by task %s", originalTask.TaskIDCommitment, task.TaskIDCommitment)
			dispatchedTask.task = task
			dispatchedTask.resChan <- false
			newResChan := make(chan bool, 1)
			dispatchedTask.resChan = newResChan
			dispatchedTask.mu.Unlock()
			log.Debugf("StartTask: waiting for task %s on node %s", task.TaskIDCommitment, node.Address)
			select {
			case res := <-newResChan:
				return res
			case <-ctx.Done():
				return false
			}
		}
		log.Debugf("StartTask: node %s is dispatching", node.Address)
		return false
	}

}

func (d *TaskDispatcher) Dispatch(ctx context.Context, task *models.InferenceTask) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			selectedNode, err := selectNodeForInferenceTask(ctx, task)

			if err == nil && selectedNode != nil {
				log.Debugf("StartTask: select node %s for task: %s", selectedNode.Address, task.TaskIDCommitment)
				ok := d.Process(ctx, task, selectedNode)
				if ok {
					log.Debugf("StartTask: dispatch task %s to node %s success", task.TaskIDCommitment, selectedNode.Address)
					return
				} else {
					log.Debugf("StartTask: dispatch task %s to node %s failed", task.TaskIDCommitment, selectedNode.Address)
				}
			} else if err != nil {
				log.Errorf("StartTask: select node for task %s error: %v", task.TaskIDCommitment, err)
			} else if selectedNode == nil {
				log.Debugf("StartTask: no available node for task %s", task.TaskIDCommitment)
			}
			randomSleep := rand.Intn(500) + 500
			time.Sleep(time.Duration(randomSleep) * time.Millisecond)
		}
	}
}

func (d *TaskDispatcher) processDispatchedTasks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case nodeAddress := <-d.nodeQueue:
			t, exists := d.taskMap.Load(nodeAddress)
			if !exists {
				log.Debugf("StartTask: node %s is not dispatching any task, skip", nodeAddress)
				continue
			}
			dispatchedTask, _ := t.(*DispatchedTask)
			log.Debugf("StartTask: start processing dispatched tasks, task %s started on node %s", dispatchedTask.task.TaskIDCommitment, dispatchedTask.node.Address)

			if time.Now().Before(dispatchedTask.createdAt.Add(time.Second)) {
				log.Debugf("StartTask: task %s is still waiting for other tasks, skip", dispatchedTask.task.TaskIDCommitment)
				d.nodeQueue <- nodeAddress
			} else {
				go func() {
					d.startTaskLimiter <- struct{}{}
					defer func() {
						<-d.startTaskLimiter
					}()

					dispatchedTask.mu.Lock()
					err := SetTaskStatusStarted(ctx, config.GetDB(), dispatchedTask.task, dispatchedTask.node)
					success := err == nil

					dispatchedTask.resChan <- success
					dispatchedTask.finished = true
					dispatchedTask.mu.Unlock()

					d.taskMap.Delete(dispatchedTask.node.Address)

					if success {
						log.Debugf("StartTask: process dispatched tasks success, task %s started on node %s", dispatchedTask.task.TaskIDCommitment, dispatchedTask.node.Address)
					} else {
						if errors.Is(err, errWrongTaskStatus) || errors.Is(err, models.ErrTaskStatusChanged) {
							log.Debugf("StartTask: process dispatched tasks failed, task %s status changed", dispatchedTask.task.TaskIDCommitment)
						} else if errors.Is(err, models.ErrNodeStatusChanged) {
							log.Debugf("StartTask: process dispatched tasks failed, node %s status changed", dispatchedTask.node.Address)
						} else {
							log.Errorf("StartTask: process dispatched tasks error: %v", err)
						}
					}
				}()
			}

		}
	}
}

func StartTaskProcesser(ctx context.Context) {
	taskDispatcher := NewTaskDispatcher()

	go taskDispatcher.processDispatchedTasks(ctx)
	go processTaskTimeouts(ctx)
	go taskDispatcher.getQueuedTasks(ctx)
}
