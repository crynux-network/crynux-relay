package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrMigrationNetworkRequired = errors.New("network is required")

type AdminMigrationResetResult struct {
	AbortedTaskCount       int64 `json:"aborted_task_count"`
	KickedOutNodeCount     int64 `json:"kicked_out_node_count"`
	DeletedNodeModelCount  int64 `json:"deleted_node_model_count"`
	DeletedNetworkNodeData int64 `json:"deleted_network_node_data"`
	DeletedCursorCount     int64 `json:"deleted_cursor_count"`
}

func AdminKickoutAllNodesAndAbortTasks(ctx context.Context, db *gorm.DB, network, abortIssuer string) (*AdminMigrationResetResult, error) {
	network = strings.TrimSpace(network)
	if network == "" {
		return nil, ErrMigrationNetworkRequired
	}
	abortIssuer = strings.TrimSpace(abortIssuer)
	if abortIssuer == "" {
		abortIssuer = getDefaultAbortIssuer()
	}

	now := time.Now()
	result := &AdminMigrationResetResult{}
	var refundCommitFuncs []func() error
	var abortedTaskCommitments []string
	var kickedOutNodes []models.Node

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tasks, err := getNonTerminalTasks(ctx, tx)
		if err != nil {
			return err
		}
		result.AbortedTaskCount = int64(len(tasks))
		refundCommitFuncs = make([]func() error, 0, len(tasks))
		abortedTaskCommitments = make([]string, 0, len(tasks))
		for i := range tasks {
			task := tasks[i]
			lastStatus := task.Status
			validatedTime := task.ValidatedTime
			if !validatedTime.Valid {
				validatedTime = sql.NullTime{Time: now, Valid: true}
			}
			commitFunc, err := refundTaskPaymentToRelayAccount(ctx, tx, task.TaskIDCommitment, task.Creator, &task.TaskFee.Int)
			if err != nil {
				return err
			}
			if err := task.Update(ctx, tx, map[string]interface{}{
				"status":         models.TaskEndAborted,
				"abort_reason":   models.TaskAbortReasonNone,
				"validated_time": validatedTime,
				"qos_score":      task.QOSScore,
			}); err != nil {
				return err
			}
			if err := emitEvent(ctx, tx, &models.TaskEndAbortedEvent{
				TaskIDCommitment: task.TaskIDCommitment,
				AbortIssuer:      abortIssuer,
				AbortReason:      models.TaskAbortReasonNone,
				LastStatus:       lastStatus,
			}); err != nil {
				return err
			}
			refundCommitFuncs = append(refundCommitFuncs, commitFunc)
			abortedTaskCommitments = append(abortedTaskCommitments, task.TaskIDCommitment)
		}

		nodes, err := getJoinedNodes(ctx, tx)
		if err != nil {
			return err
		}
		result.KickedOutNodeCount = int64(len(nodes))
		kickedOutNodes = append(kickedOutNodes, nodes...)
		for i := range nodes {
			node := nodes[i]
			if err := emitEvent(ctx, tx, &models.NodeQuitEvent{
				NodeAddress:             node.Address,
				BlockchainTransactionID: 0,
				Network:                 node.Network,
			}); err != nil {
				return err
			}
			if err := emitEvent(ctx, tx, &models.NodeKickedOutEvent{
				NodeAddress: node.Address,
				Network:     node.Network,
			}); err != nil {
				return err
			}
		}

		modelDelete := tx.WithContext(ctx).Where("1 = 1").Delete(&models.NodeModel{})
		if modelDelete.Error != nil {
			return modelDelete.Error
		}
		result.DeletedNodeModelCount = modelDelete.RowsAffected

		nodeUpdate := tx.WithContext(ctx).Model(&models.Node{}).
			Where("status <> ?", models.NodeStatusQuit).
			Updates(map[string]interface{}{
				"status":                     models.NodeStatusQuit,
				"current_task_id_commitment": sql.NullString{Valid: false},
				"stake_amount":               models.BigInt{Int: *big.NewInt(0)},
			})
		if nodeUpdate.Error != nil {
			return nodeUpdate.Error
		}

		nodeDataDelete := tx.WithContext(ctx).Where("1 = 1").Delete(&models.NetworkNodeData{})
		if nodeDataDelete.Error != nil {
			return nodeDataDelete.Error
		}
		result.DeletedNetworkNodeData = nodeDataDelete.RowsAffected

		if err := tx.WithContext(ctx).Where("1 = 1").Delete(&models.NodeNameCount{}).Error; err != nil {
			return err
		}

		cursorDelete := tx.WithContext(ctx).Where("network = ?", network).Delete(&models.BlockchainCursor{})
		if cursorDelete.Error != nil {
			return cursorDelete.Error
		}
		result.DeletedCursorCount = cursorDelete.RowsAffected

		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, commitFunc := range refundCommitFuncs {
		if err := commitFunc(); err != nil {
			return nil, err
		}
	}
	for _, taskIDCommitment := range abortedTaskCommitments {
		deleteRunningTaskSnapshot(taskIDCommitment)
	}
	for _, node := range kickedOutNodes {
		UpdateMaxStaking(node.Address, big.NewInt(0))
	}
	if err := RefreshNodeNameCountCache(ctx, db); err != nil {
		return nil, err
	}

	return result, nil
}

func getNonTerminalTasks(ctx context.Context, db *gorm.DB) ([]models.InferenceTask, error) {
	statuses := []models.TaskStatus{
		models.TaskQueued,
		models.TaskStarted,
		models.TaskParametersUploaded,
		models.TaskErrorReported,
		models.TaskScoreReady,
		models.TaskValidated,
		models.TaskGroupValidated,
	}
	var tasks []models.InferenceTask
	if err := db.WithContext(ctx).
		Model(&models.InferenceTask{}).
		Where("status IN ?", statuses).
		Order("id").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func getJoinedNodes(ctx context.Context, db *gorm.DB) ([]models.Node, error) {
	var nodes []models.Node
	if err := db.WithContext(ctx).
		Model(&models.Node{}).
		Where("status <> ?", models.NodeStatusQuit).
		Order("id").
		Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}
