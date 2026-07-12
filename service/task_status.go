package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
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
	ErrWrongNodeCurrentTask = errors.New("node current task is wrong")
)

func CreateTask(ctx context.Context, db *gorm.DB, task *models.InferenceTask) error {
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := task.Create(ctx, tx); err != nil {
			return err
		}
		if err := models.AddTotalTask(ctx, tx); err != nil {
			return err
		}
		commitFunc, err := chargeTaskFromRelayAccount(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
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
	metrics.TasksCreated.WithLabelValues(metrics.TaskTypeLabel(task.TaskType), task.Creator, metrics.VramTierLabel(task.MinVRAM)).Inc()
	return nil
}

// MarkTaskDelivered records the first time the selected node fetches the task.
// The write is a single conditional UPDATE so concurrent fetches record the
// delivery exactly once.
func MarkTaskDelivered(ctx context.Context, db *gorm.DB, task *models.InferenceTask) error {
	if task.DeliveredTime.Valid {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	deliveredTime := sql.NullTime{Time: time.Now(), Valid: true}
	result := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
		Where("id = ?", task.ID).
		Where("delivered_time IS NULL").
		Update("delivered_time", deliveredTime)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		task.DeliveredTime = deliveredTime
		metrics.TasksDelivered.Inc()
	}
	return nil
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
	startTime := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := task.Update(ctx, tx, map[string]interface{}{
			"selected_node":  node.Address,
			"start_time":     sql.NullTime{Time: startTime, Valid: true},
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
	metrics.TasksDispatched.WithLabelValues(metrics.TaskTypeLabel(task.TaskType)).Inc()
	if task.CreateTime.Valid {
		metrics.TaskQueueWaitSeconds.WithLabelValues(metrics.TaskTypeLabel(task.TaskType), metrics.VramTierLabel(task.MinVRAM)).Observe(startTime.Sub(task.CreateTime.Time).Seconds())
	}
	if err := captureRunningTaskSnapshot(ctx, db, &task, &node); err != nil {
		log.Errorf("SetTaskStatusStarted: failed to capture running task snapshot, task: %s, node: %s, error: %v", task.TaskIDCommitment, node.Address, err)
	}
	if err := captureTaskTraceStartSnapshot(ctx, db, &task, &node); err != nil {
		log.Errorf("SetTaskStatusStarted: failed to capture task trace start snapshot, task: %s, node: %s, error: %v", task.TaskIDCommitment, node.Address, err)
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
		return nil, ErrWrongNodeCurrentTask
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

	scoreReadyTime := time.Now()
	if err := db.Transaction(func(tx *gorm.DB) error {
		err = task.Update(ctx, tx, map[string]interface{}{
			"status":           models.TaskScoreReady,
			"score":            task.Score,
			"score_ready_time": sql.NullTime{Time: scoreReadyTime, Valid: true},
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
	if task.StartTime.Valid {
		metrics.TaskExecutionSeconds.WithLabelValues(metrics.TaskTypeLabel(task.TaskType), metrics.VramTierLabel(task.MinVRAM)).Observe(scoreReadyTime.Sub(task.StartTime.Time).Seconds())
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
	metrics.TasksErrorReported.Inc()
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

	var qosTraceEvents []NodeQosTraceInput
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err = task.Update(ctx, tx, map[string]interface{}{
			"status":         models.TaskGroupValidated,
			"validated_time": sql.NullTime{Time: time.Now(), Valid: true},
			"qos_score":      task.QOSScore,
		}); err != nil {
			return err
		}
		if task.QOSScore.Valid {
			before := CaptureNodeQosTraceValues(node)
			if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
				return err
			}
			eventType, validationRank := BuildValidationGroupQosTraceMetadata(&task)
			if eventType != "" {
				taskQosScore := uint64(task.QOSScore.Int64)
				qosTraceEvents = append(qosTraceEvents, NodeQosTraceInput{
					NodeAddress:      node.Address,
					TaskIDCommitment: task.TaskIDCommitment,
					EventType:        eventType,
					TaskQosScore:     &taskQosScore,
					ValidationRank:   validationRank,
					Before:           before,
					After:            CaptureNodeQosTraceValues(node),
				})
			}
		}

		return emitEvent(ctx, tx, &models.TaskValidatedEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode})
	}); err != nil {
		return err
	}
	for _, event := range qosTraceEvents {
		RecordNodeQosTrace(event)
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

	evidence, evidenceComplete, err := buildSlashEvidence(ctx, db, &task, node)
	if err != nil {
		return err
	}
	passiveSlashMode := config.GetConfig().Task.PassiveSlashMode != nil && *config.GetConfig().Task.PassiveSlashMode
	return setTaskStatusEndInvalidatedWithEvidence(ctx, db, originTask, node, evidence, evidenceComplete, passiveSlashMode)
}

func SetTaskStatusEndInvalidatedWithEvidence(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask, evidence *models.SlashEvidence, evidenceComplete bool) error {
	task := *originTask
	if task.Status != models.TaskScoreReady && task.Status != models.TaskEndAborted && task.Status != models.TaskErrorReported {
		return errWrongTaskStatus
	}

	node, err := checkTaskSelectedNode(ctx, db, &task)
	if err != nil {
		return err
	}
	passiveSlashMode := config.GetConfig().Task.PassiveSlashMode != nil && *config.GetConfig().Task.PassiveSlashMode
	return setTaskStatusEndInvalidatedWithEvidence(ctx, db, originTask, node, evidence, evidenceComplete, passiveSlashMode)
}

func setTaskStatusEndInvalidatedWithEvidence(ctx context.Context, db *gorm.DB, originTask *models.InferenceTask, node *models.Node, evidence *models.SlashEvidence, evidenceComplete bool, passiveSlashMode bool) error {
	task := *originTask
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := task.Update(ctx, tx, map[string]interface{}{
			"status":         models.TaskEndInvalidated,
			"validated_time": sql.NullTime{Time: time.Now(), Valid: true},
			"qos_score":      0,
		}); err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.TaskEndInvalidatedEvent{TaskIDCommitment: task.TaskIDCommitment, SelectedNode: task.SelectedNode}); err != nil {
			return err
		}
		if passiveSlashMode {
			if err := createPendingSlash(ctx, tx, &task, node, evidence, evidenceComplete); err != nil {
				return err
			}
			return nodeFinishTask(ctx, tx, node)
		}
		_, err := SlashNode(ctx, tx, node, task.TaskIDCommitment, evidence)
		return err
	}); err != nil {
		return err
	}
	metrics.TasksTerminal.WithLabelValues("invalidated", metrics.TaskTypeLabel(task.TaskType), metrics.VramTierLabel(task.MinVRAM)).Inc()
	deleteRunningTaskSnapshot(task.TaskIDCommitment)
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

	healthBoostMetrics := nodeHealthMetrics{}
	logHealthBoost := false
	var qosTraceEvents []NodeQosTraceInput
	if err := db.Transaction(func(tx *gorm.DB) error {
		commitFunc, err := refundTaskPaymentToRelayAccount(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
		if err != nil {
			return err
		}
		if task.QOSScore.Valid {
			before := CaptureNodeQosTraceValues(node)
			if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
				return err
			}
			eventType, validationRank := BuildValidationGroupQosTraceMetadata(&task)
			if eventType != "" {
				taskQosScore := uint64(task.QOSScore.Int64)
				qosTraceEvents = append(qosTraceEvents, NodeQosTraceInput{
					NodeAddress:      node.Address,
					TaskIDCommitment: task.TaskIDCommitment,
					EventType:        eventType,
					TaskQosScore:     &taskQosScore,
					ValidationRank:   validationRank,
					Before:           before,
					After:            CaptureNodeQosTraceValues(node),
				})
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
		healthBoostMetrics = calculateBoostNodeHealthMetrics(node)
		logHealthBoost = shouldLogHealthBoost(healthBoostMetrics)
		before := CaptureNodeQosTraceValues(node)
		if err := ApplyHealthBoost(ctx, tx, node); err != nil {
			return err
		}
		qosTraceEvents = append(qosTraceEvents, NodeQosTraceInput{
			NodeAddress:      node.Address,
			TaskIDCommitment: task.TaskIDCommitment,
			EventType:        QosTraceEventValidationGroupMatchedBoost,
			Before:           before,
			After:            CaptureNodeQosTraceValues(node),
		})
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
	metrics.TasksTerminal.WithLabelValues("group_refund", metrics.TaskTypeLabel(task.TaskType), metrics.VramTierLabel(task.MinVRAM)).Inc()
	updateLoadedModels(&task, node)
	if logHealthBoost {
		logHealthBoostNodeHealthEvent(node, &task, healthBoostMetrics)
	}
	for _, event := range qosTraceEvents {
		RecordNodeQosTrace(event)
	}
	deleteRunningTaskSnapshot(task.TaskIDCommitment)
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
	timeoutPenaltyMetrics := nodeHealthMetrics{}
	logTimeoutPenalty := false
	var timeoutPenaltyNode *models.Node
	var qosTraceEvents []NodeQosTraceInput
	if err := db.Transaction(func(tx *gorm.DB) error {
		commitFunc, err := refundTaskPaymentToRelayAccount(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
		if err != nil {
			return err
		}

		if err := task.Update(ctx, tx, newTask); err != nil {
			return err
		}

		if len(task.SelectedNode) > 0 {
			node, err := checkTaskSelectedNode(ctx, db, &task)
			if errors.Is(err, ErrWrongNodeCurrentTask) {
				log.Errorf("TaskEndAborted: node current task is wrong, task: %s, node: %s", task.TaskIDCommitment, task.SelectedNode)
			} else if err != nil {
				return err
			} else {
				if task.QOSScore.Valid {
					before := CaptureNodeQosTraceValues(node)
					if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
						return err
					}
					eventType, validationRank := BuildValidationGroupQosTraceMetadata(&task)
					if eventType != "" {
						taskQosScore := uint64(task.QOSScore.Int64)
						qosTraceEvents = append(qosTraceEvents, NodeQosTraceInput{
							NodeAddress:      node.Address,
							TaskIDCommitment: task.TaskIDCommitment,
							EventType:        eventType,
							TaskQosScore:     &taskQosScore,
							ValidationRank:   validationRank,
							Before:           before,
							After:            CaptureNodeQosTraceValues(node),
						})
					}
				}
				// Apply health penalty on timeout when the node never submitted a result
				if task.AbortReason == models.TaskAbortTimeout && !task.ScoreReadyTime.Valid {
					timeoutPenaltyMetrics = calculatePenaltyNodeHealthMetrics(node)
					before := CaptureNodeQosTraceValues(node)
					if err := ApplyHealthPenalty(ctx, tx, node); err != nil {
						return err
					}
					qosTraceEvents = append(qosTraceEvents, NodeQosTraceInput{
						NodeAddress:      node.Address,
						TaskIDCommitment: task.TaskIDCommitment,
						EventType:        QosTraceEventTaskTimeoutPenalty,
						Before:           before,
						After:            CaptureNodeQosTraceValues(node),
					})
					timeoutPenaltyNode = node
					logTimeoutPenalty = true
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
	metrics.TasksAborted.WithLabelValues(metrics.AbortReasonLabel(task.AbortReason), metrics.AbortStatusLabel(lastStatus, &task), metrics.TaskTypeLabel(task.TaskType), metrics.VramTierLabel(task.MinVRAM)).Inc()
	if logTimeoutPenalty && timeoutPenaltyNode != nil {
		logTaskTimeoutNodeHealthEvent(timeoutPenaltyNode, &task, timeoutPenaltyMetrics)
	}
	for _, event := range qosTraceEvents {
		RecordNodeQosTrace(event)
	}
	deleteRunningTaskSnapshot(task.TaskIDCommitment)
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

	healthBoostMetrics := nodeHealthMetrics{}
	logHealthBoost := false
	var qosTraceEvents []NodeQosTraceInput
	if err := db.Transaction(func(tx *gorm.DB) error {
		var commitFuncs []func() error
		for _, payment := range payments {
			commitFunc, err := sendTaskIncome(ctx, tx, payment.taskIDCommitment, payment.address, payment.payment, task.TaskType, payment.network)
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

		healthBoostMetrics = calculateBoostNodeHealthMetrics(node)
		logHealthBoost = shouldLogHealthBoost(healthBoostMetrics)
		before := CaptureNodeQosTraceValues(node)
		if err := ApplyHealthBoost(ctx, tx, node); err != nil {
			return err
		}
		qosTraceEvents = append(qosTraceEvents, NodeQosTraceInput{
			NodeAddress:      node.Address,
			TaskIDCommitment: task.TaskIDCommitment,
			EventType:        QosTraceEventTaskResultUploadSuccessBoost,
			Before:           before,
			After:            CaptureNodeQosTraceValues(node),
		})

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
	terminalStatusLabel := "success"
	if status == models.TaskEndGroupSuccess {
		terminalStatusLabel = "group_success"
	}
	metrics.TasksTerminal.WithLabelValues(terminalStatusLabel, metrics.TaskTypeLabel(task.TaskType), metrics.VramTierLabel(task.MinVRAM)).Inc()
	updateLoadedModels(&task, node)
	if logHealthBoost {
		logHealthBoostNodeHealthEvent(node, &task, healthBoostMetrics)
	}
	for _, event := range qosTraceEvents {
		RecordNodeQosTrace(event)
	}
	deleteRunningTaskSnapshot(task.TaskIDCommitment)
	*originTask = task
	return nil
}
