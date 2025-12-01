package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	errWrongTaskStatus      = errors.New("illegal previous task status")
	errWrongNodeCurrentTask = errors.New("node current task is wrong")
)

func CreateTask(ctx context.Context, db *gorm.DB, task *models.InferenceTask) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := task.Create(ctx, tx); err != nil {
			return err
		}
		if err := models.AddTotalTask(ctx, tx); err != nil {
			return err
		}
		commitFunc, err := SpendTaskQuota(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
		if err != nil {
			return err
		}
		if err := commitFunc(); err != nil {
			return err
		}
		return nil
	})
}

func isNodeVersionValidForTask(node *models.Node, task *models.InferenceTask) bool {
	taskVersionNumbers := task.VersionNumbers()
	return node.MajorVersion == taskVersionNumbers[0] && (node.MinorVersion > taskVersionNumbers[1] || (node.MinorVersion == taskVersionNumbers[1] && node.PatchVersion >= taskVersionNumbers[2]))
}

func SetTaskStatusStarted(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask, originNode *models.Node) error {
	task := *originTask
	node := *originNode

	if task.Status != models.TaskQueued {
		return errWrongTaskStatus
	}
	if err := node.Sync(ctx, db); err != nil {
		return err
	}
	if !isNodeVersionValidForTask(&node, &task) {
		return errors.New("node version is not compatible with task")
	}

	var inUseModelIDs []string
	for _, model := range node.Models {
		if model.InUse {
			inUseModelIDs = append(inUseModelIDs, model.ModelID)
		}
	}

	// start inference task
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := task.Update(ctx, tx, map[string]interface{}{
			"selected_node":  node.Address,
			"start_time":     sql.NullTime{Time: time.Now(), Valid: true},
			"status":         models.TaskStarted,
			"model_swtiched": !isSameModels(inUseModelIDs, task.ModelIDs),
		}); err != nil {
			return err
		}

		if err := nodeStartTask(ctx, tx, &node, task.TaskIDCommitment, task.ModelIDs); err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.TaskStartedEvent{
			TaskIDCommitment: task.TaskIDCommitment,
			SelectedNode:     node.Address,
		})
	})
	if err != nil {
		return err
	}

	*originTask = task
	*originNode = node

	// start download tasks
	localModelSet := make(map[string]models.NodeModel)
	for _, model := range node.Models {
		localModelSet[model.ModelID] = model
	}

	for _, modelID := range task.ModelIDs {
		download := false
		if _, ok := localModelSet[modelID]; !ok {
			emitEvent(ctx, db, &models.DownloadModelEvent{
				NodeAddress: node.Address,
				ModelID:     modelID,
				TaskType:    task.TaskType,
			})
			download = true
		}

		count, err := countAvailableNodesWithModelID(ctx, db, modelID)
		if err != nil {
			return err
		}
		if count < 3 {
			downloadNodes, err := selectNodesForDownloadTask(ctx, &task, modelID, 10-int(count))
			if err != nil {
				return err
			}
			if len(downloadNodes) > 0 {
				for _, downloadNode := range downloadNodes {
					if !download || node.Address != downloadNode.Address {
						emitEvent(ctx, db, &models.DownloadModelEvent{
							NodeAddress: downloadNode.Address,
							ModelID:     modelID,
							TaskType:    task.TaskType,
						})
					}
				}
			}
		}
	}
	return nil
}

func checkTaskSelectedNode(ctx context.Context, db *gorm.DB, task *models.InferenceTask) (*models.Node, error) {
	node, err := models.GetNodeByAddress(ctx, db, task.SelectedNode)
	if err != nil {
		return nil, err
	}
	if !(node.CurrentTaskIDCommitment.Valid && node.CurrentTaskIDCommitment.String == task.TaskIDCommitment) {
		return nil, errWrongNodeCurrentTask
	}
	return node, nil
}

func SetTaskStatusScoreReady(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	if task.Status != models.TaskStarted {
		return errWrongTaskStatus
	}
	_, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		err = task.Update(ctx, tx, map[string]interface{}{
			"status":           models.TaskScoreReady,
			"score":            task.Score,
			"score_ready_time": sql.NullTime{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.TaskScoreReadyEvent{
			TaskIDCommitment: task.TaskIDCommitment,
			SelectedNode:     task.SelectedNode,
			Score:            task.Score,
		})
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusErrorReported(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	if task.Status != models.TaskStarted {
		return errWrongTaskStatus
	}
	_, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		err = task.Update(ctx, tx, map[string]interface{}{
			"status":           models.TaskErrorReported,
			"task_error":       task.TaskError,
			"score_ready_time": sql.NullTime{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.TaskErrorReportedEvent{
			TaskIDCommitment: task.TaskIDCommitment,
			SelectedNode:     task.SelectedNode,
			TaskError:        task.TaskError,
		})
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusValidated(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	if task.Status != models.TaskScoreReady {
		return errWrongTaskStatus
	}
	_, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		err = task.Update(ctx, tx, map[string]interface{}{
			"status":         models.TaskValidated,
			"validated_time": sql.NullTime{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.TaskValidatedEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode})
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusGroupValidated(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	if task.Status != models.TaskScoreReady {
		return errWrongTaskStatus
	}
	node, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err = task.Update(ctx, tx, map[string]interface{}{
			"status":         models.TaskGroupValidated,
			"validated_time": sql.NullTime{Time: time.Now(), Valid: true},
			"qos_score":      task.QOSScore,
		}); err != nil {
			return err
		}
		if task.QOSScore.Valid {
			if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
				return err
			}
		}

		return emitEvent(ctx, tx, &models.TaskValidatedEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode})
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusEndInvalidated(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	if task.Status != models.TaskScoreReady && task.Status != models.TaskEndAborted && task.Status != models.TaskErrorReported {
		return errWrongTaskStatus
	}

	node, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		err = task.Update(ctx, tx, map[string]interface{}{
			"status":         models.TaskEndInvalidated,
			"validated_time": sql.NullTime{Time: time.Now(), Valid: true},
			"qos_score":      0,
		})
		if err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.TaskEndInvalidatedEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode}); err != nil {
			return err
		}
		return nodeSlash(ctx, tx, node)
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusEndGroupRefund(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	if task.Status != models.TaskScoreReady {
		return errWrongTaskStatus
	}

	node, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		commitFunc, err := RefundTaskQuota(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
		if err != nil {
			return err
		}
		if task.QOSScore.Valid {
			if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
				return err
			}
		}

		err = task.Update(ctx, tx, map[string]interface{}{
			"status":         models.TaskEndGroupRefund,
			"validated_time": sql.NullTime{Time: time.Now(), Valid: true},
			"qos_score":      task.QOSScore,
		})
		if err != nil {
			return err
		}
		if err := nodeFinishTask(ctx, tx, node); err != nil {
			return err
		}
		err = emitEvent(ctx, tx, &models.TaskEndGroupRefundEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode})
		if err != nil {
			return err
		}
		if err := commitFunc(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusEndAborted(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask, aboutIssuer string) error {
	task := *originTask
	if task.Status == models.TaskEndAborted {
		return nil
	}
	if task.Status == models.TaskEndSuccess || task.Status == models.TaskEndInvalidated || task.Status == models.TaskEndGroupSuccess || task.Status == models.TaskEndGroupRefund {
		return errWrongTaskStatus
	}
	lastStatus := task.Status

	newTask := map[string]interface{}{
		"status":         models.TaskEndAborted,
		"abort_reason":   task.AbortReason,
		"validated_time": task.ValidatedTime,
		"qos_score":      task.QOSScore,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		commitFunc, err := RefundTaskQuota(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
		if err != nil {
			return err
		}

		if err := task.Update(ctx, tx, newTask); err != nil {
			return err
		}

		if len(task.SelectedNode) > 0 {
			node, err := checkTaskSelectedNode(ctx, db, &task)
			if errors.Is(err, errWrongNodeCurrentTask) {
				log.Errorf("TaskEndAborted: node current task is wrong, task: %s, node: %s", task.TaskIDCommitment, task.SelectedNode)
			} else if err != nil {
				return err
			} else {
				if task.QOSScore.Valid {
					if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
						return err
					}
				}
				if err := nodeFinishTask(ctx, tx, node); err != nil {
					return err
				}
			}
		}

		err = emitEvent(ctx, tx, &models.TaskEndAbortedEvent{
			TaskIDCommitment: task.TaskIDCommitment,
			AbortIssuer:      aboutIssuer,
			AbortReason:      task.AbortReason,
			LastStatus:       lastStatus,
		})
		if err != nil {
			return err
		}
		if err := commitFunc(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}

func SetTaskStatusEndSuccess(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask) error {
	task := *originTask
	node, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}

	tasks, err := models.GetTaskGroupByTaskID(ctx, db, task.TaskID)
	if err != nil {
		return err
	}
	status := models.TaskEndSuccess
	type taskPayment struct {
		taskIDCommitment string
		address          string
		payment          *big.Int
		network          string
	}
	payments := make([]taskPayment, 0)
	if len(tasks) > 1 {
		status = models.TaskEndGroupSuccess
		// calculate each task's payment
		var totalScore uint64 = 0
		var validTasks []models.InferenceTask
		var validNodeAddresses []string
		for _, t := range tasks {
			if t.Status == models.TaskGroupValidated || t.Status == models.TaskEndGroupRefund {
				// qos of task in group validated or group refunded task is valid
				totalScore += uint64(t.QOSScore.Int64)
				validTasks = append(validTasks, t)
				validNodeAddresses = append(validNodeAddresses, t.SelectedNode)
			}
		}
		validNodes, err := models.GetNodesByAddresses(ctx, db, validNodeAddresses)
		if err != nil {
			return err
		}
		nodeNetworkMap := make(map[string]string)
		for _, node := range validNodes {
			nodeNetworkMap[node.Address] = node.Network
		}
		totalRem := big.NewInt(0)
		for i, t := range validTasks {
			payment := big.NewInt(0).Mul(&t.TaskFee.Int, big.NewInt(0).SetInt64(t.QOSScore.Int64))
			payment, rem := big.NewInt(0).QuoRem(payment, big.NewInt(0).SetUint64(totalScore), big.NewInt(0))
			totalRem.Add(totalRem, rem)
			if i == len(validTasks)-1 {
				payment.Add(payment, totalRem)
			}
			payments = append(payments, taskPayment{
				taskIDCommitment: t.TaskIDCommitment,
				address:          t.SelectedNode,
				payment:          payment,
				network:          nodeNetworkMap[t.SelectedNode],
			})
		}

	} else {
		payments = append(payments, taskPayment{
			taskIDCommitment: task.TaskIDCommitment,
			address:          task.SelectedNode,
			payment:          &task.TaskFee.Int,
			network:          node.Network,
		})
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		var commitFuncs []func() error
		for _, payment := range payments {
			commitFunc, err := sendTaskFee(ctx, tx, payment.taskIDCommitment, payment.address, payment.payment, task.TaskType, payment.network)
			if err != nil {
				return err
			}
			commitFuncs = append(commitFuncs, commitFunc)
		}

		err = task.Update(ctx, tx, map[string]interface{}{
			"status":               status,
			"result_uploaded_time": sql.NullTime{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return err
		}

		if err := nodeFinishTask(ctx, tx, node); err != nil {
			return err
		}

		if status == models.TaskEndSuccess {
			err = emitEvent(ctx, tx, &models.TaskEndSuccessEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode})
		} else {
			err = emitEvent(ctx, tx, &models.TaskEndGroupSuccessEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode})
		}
		if err != nil {
			return err
		}
		for _, commitFunc := range commitFuncs {
			if err := commitFunc(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	*originTask = task
	return nil
}
