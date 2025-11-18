package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func SetNodeStatusJoin(ctx context.Context, db *gorm.DB, node *models.Node, modelIDs []string) error {
	stakingInfo, err := blockchain.GetStakingInfo(ctx, common.HexToAddress(node.Address), node.Network)
	if err != nil {
		return err
	}
	stakingAmount := new(big.Int).Add(stakingInfo.StakedBalance, stakingInfo.StakedCredits)
	if stakingAmount.Cmp(&node.StakeAmount.Int) != 0 {
		return errors.New("staking amount mismatch")
	}
	userStakingAmount := GetUserStakeAmountOfNode(node.Address, node.Network)
	totalStakingAmount := big.NewInt(0).Add(stakingAmount, userStakingAmount)
	delegatorShare, err := blockchain.GetNodeDelegatorShare(ctx, common.HexToAddress(node.Address), node.Network)
	if err != nil {
		return err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		node.Status = models.NodeStatusAvailable
		node.JoinTime = time.Now()
		node.DelegatorShare = delegatorShare
		if err := node.Save(ctx, tx); err != nil {
			return err
		}
		var nodeModels []models.NodeModel
		for _, modelID := range modelIDs {
			model := models.NodeModel{NodeAddress: node.Address, ModelID: modelID, InUse: false}
			nodeModels = append(nodeModels, model)
		}
		if err := models.CreateNodeModels(ctx, tx, nodeModels); err != nil {
			return err
		}
		networkNodeData := models.NetworkNodeData{
			Address:   node.Address,
			CardModel: node.GPUName,
			VRam:      int(node.GPUVram),
			QoS:       node.QOSScore,
			Staking:   models.BigInt{Int: *totalStakingAmount},
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			DoUpdates: clause.AssignmentColumns([]string{"card_model", "v_ram", "qo_s", "staking", "updated_at"}),
		}).Create(&networkNodeData).Error; err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.NodeJoinEvent{NodeAddress: node.Address}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	UpdateMaxStaking(node.Address, &node.StakeAmount.Int)
	return nil
}

func SetNodeStatusQuit(ctx context.Context, db *gorm.DB, node *models.Node, slashed bool) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		// delete all node local models
		err := tx.Where("node_address = ?", node.Address).Delete(&models.NodeModel{}).Error
		if err != nil {
			return err
		}

		if err := node.Update(ctx, tx, map[string]interface{}{
			"status":                     models.NodeStatusQuit,
			"current_task_id_commitment": sql.NullString{Valid: false},
			"stake_amount":               models.BigInt{Int: *big.NewInt(0)},
		}); err != nil {
			return err
		}
		UpdateMaxStaking(node.Address, big.NewInt(0))
		var blockchainTransaction *models.BlockchainTransaction
		if slashed {
			blockchainTransaction, err = blockchain.QueueSlashStaking(ctx, tx, common.HexToAddress(node.Address), node.Network)
			if err != nil {
				return err
			}
		} else {
			blockchainTransaction, err = blockchain.QueueUnstake(ctx, tx, common.HexToAddress(node.Address), node.Network)
			if err != nil {
				return err
			}
		}
		if err := emitEvent(ctx, tx, &models.NodeQuitEvent{NodeAddress: node.Address, BlockchainTransactionID: blockchainTransaction.ID}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func nodeStartTask(ctx context.Context, db *gorm.DB, node *models.Node, taskIDCommitment string, taskModelIDs []string) error {
	if node.Status != models.NodeStatusAvailable || node.CurrentTaskIDCommitment.Valid {
		return errors.New("node is not available")
	}

	newModels := make([]models.NodeModel, 0)
	unusedModels := make([]models.NodeModel, 0)

	localModelSet := make(map[string]models.NodeModel)
	for _, model := range node.Models {
		localModelSet[model.ModelID] = model
	}
	for _, modelID := range taskModelIDs {
		if model, ok := localModelSet[modelID]; !ok {
			newModel := models.NodeModel{NodeAddress: node.Address, ModelID: modelID, InUse: true}
			newModels = append(newModels, newModel)
		} else if !model.InUse {
			model.InUse = true
			newModels = append(newModels, model)
		}
	}
	taskModelIDSet := make(map[string]struct{})
	for _, modelID := range taskModelIDs {
		taskModelIDSet[modelID] = struct{}{}
	}
	for _, model := range node.Models {
		_, ok := taskModelIDSet[model.ModelID]
		if model.InUse && !ok {
			model.InUse = false
			unusedModels = append(unusedModels, model)
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := node.Update(ctx, tx, map[string]interface{}{
			"status":                     models.NodeStatusBusy,
			"current_task_id_commitment": sql.NullString{String: taskIDCommitment, Valid: true},
		}); err != nil {
			return err
		}

		for _, model := range newModels {
			if err := model.Save(ctx, tx); err != nil {
				return err
			}
		}
		for _, model := range unusedModels {
			if err := model.Save(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func nodeFinishTask(ctx context.Context, db *gorm.DB, node *models.Node) error {
	if !(node.Status == models.NodeStatusBusy || node.Status == models.NodeStatusPendingPause || node.Status == models.NodeStatusPendingQuit) {
		return errors.New("illegal node status")
	}
	if !node.CurrentTaskIDCommitment.Valid {
		return errors.New("task id commitment is not valid")
	}
	taskIDCommitment := node.CurrentTaskIDCommitment.String
	kickout, err := shouldKickoutNode(ctx, node)
	if err != nil {
		return err
	}
	if kickout {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := SetNodeStatusQuit(ctx, db, node, false); err != nil {
				return err
			}
			return emitEvent(ctx, db, &models.NodeKickedOutEvent{NodeAddress: node.Address, TaskIDCommitment: taskIDCommitment})
		})
	}

	switch node.Status {
	case models.NodeStatusBusy:
		return node.Update(ctx, db, map[string]interface{}{
			"status":                     models.NodeStatusAvailable,
			"current_task_id_commitment": sql.NullString{Valid: false},
		})
	case models.NodeStatusPendingQuit:
		return SetNodeStatusQuit(ctx, db, node, false)
	case models.NodeStatusPendingPause:
		return node.Update(ctx, db, map[string]interface{}{
			"status":                     models.NodeStatusPaused,
			"current_task_id_commitment": sql.NullString{Valid: false},
		})
	}
	return nil
}

func nodeSlash(ctx context.Context, db *gorm.DB, node *models.Node) error {
	if !(node.Status == models.NodeStatusBusy || node.Status == models.NodeStatusPendingPause || node.Status == models.NodeStatusPendingQuit) {
		return errors.New("illegal node status")
	}
	if !node.CurrentTaskIDCommitment.Valid {
		return errors.New("task id commitment is not valid")
	}
	taskIDCommitment := node.CurrentTaskIDCommitment.String
	return db.Transaction(func(tx *gorm.DB) error {
		if err := SetNodeStatusQuit(ctx, db, node, true); err != nil {
			return err
		}
		return emitEvent(ctx, db, &models.NodeSlashedEvent{NodeAddress: node.Address, TaskIDCommitment: taskIDCommitment})
	})
}

func updateNodeQosScore(ctx context.Context, db *gorm.DB, node *models.Node, qos uint64) error {
	qosScore, err := getNodeTaskQosScore(node, qos)
	if err != nil {
		return err
	}
	return node.Update(ctx, db, map[string]interface{}{
		"qos_score": qosScore,
	})
}
