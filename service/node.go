package service

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	getStakingInfo        = blockchain.GetStakingInfo
	getNodeDelegatorShare = blockchain.GetNodeDelegatorShare
	getNodeStakingInfos   = blockchain.GetNodeStakingInfos
)

type chainDelegation struct {
	DelegatorAddress string
	Amount           *big.Int
}

func SetNodeStatusJoin(ctx context.Context, db *gorm.DB, node *models.Node, modelIDs []string) error {
	nodeAddress := common.HexToAddress(node.Address)
	stakingInfo, err := getStakingInfo(ctx, nodeAddress, node.Network)
	if err != nil {
		return err
	}
	stakingAmount := new(big.Int).Add(stakingInfo.StakedBalance, stakingInfo.StakedCredits)
	if stakingAmount.Cmp(&node.StakeAmount.Int) != 0 {
		return errors.New("staking amount mismatch")
	}
	delegatorShare, err := getNodeDelegatorShare(ctx, nodeAddress, node.Network)
	if err != nil {
		return err
	}
	delegatorAddresses, delegationAmounts, err := getNodeStakingInfos(ctx, nodeAddress, node.Network)
	if err != nil {
		return err
	}
	chainDelegations, delegatedStakingAmount, err := normalizeChainDelegations(delegatorAddresses, delegationAmounts)
	if err != nil {
		return err
	}
	totalStakingAmount := big.NewInt(0).Add(stakingAmount, delegatedStakingAmount)

	err = db.Transaction(func(tx *gorm.DB) error {
		node.Status = models.NodeStatusAvailable
		node.JoinTime = time.Now()
		node.HealthBase = 1.0
		node.HealthUpdatedAt = sql.NullTime{Time: time.Now(), Valid: true}
		node.DelegatorShare = delegatorShare
		if err := node.Save(ctx, tx); err != nil {
			return err
		}
		if err := syncNodeDelegationsFromChainTx(tx, node.Address, node.Network, chainDelegations); err != nil {
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
			Address:         node.Address,
			CardModel:       node.GPUName,
			VRam:            int(node.GPUVram),
			QoS:             node.QOSScore,
			Staking:         models.BigInt{Int: *totalStakingAmount},
			HealthBase:      node.HealthBase,
			HealthUpdatedAt: node.HealthUpdatedAt,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			DoUpdates: clause.AssignmentColumns([]string{"card_model", "v_ram", "qo_s", "staking", "health_base", "health_updated_at", "updated_at"}),
		}).Create(&networkNodeData).Error; err != nil {
			return err
		}
		if err := IncrementNodeNameCountTx(ctx, tx, node); err != nil {
			return err
		}
		if err := emitEvent(ctx, tx, &models.NodeJoinEvent{NodeAddress: node.Address, Network: node.Network}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	ApplyNodeNameCountDeltaToCache(node.GPUName, node.GPUVram, BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion), 1)
	applyNodeDelegationsToCache(node.Address, node.Network, chainDelegations)
	SetDelegatorShare(node.Address, node.Network, delegatorShare)
	UpdateMaxStaking(node.Address, totalStakingAmount)
	LogNodeStatusChange(node, "join")
	return nil
}

func normalizeChainDelegations(delegatorAddresses []common.Address, amounts []*big.Int) ([]chainDelegation, *big.Int, error) {
	if len(delegatorAddresses) != len(amounts) {
		return nil, nil, fmt.Errorf("delegated staking info length mismatch: %d addresses, %d amounts", len(delegatorAddresses), len(amounts))
	}
	delegations := make([]chainDelegation, 0, len(delegatorAddresses))
	total := big.NewInt(0)
	for i, delegatorAddress := range delegatorAddresses {
		amount := amounts[i]
		if amount == nil || amount.Sign() == 0 {
			continue
		}
		amountCopy := big.NewInt(0).Set(amount)
		delegations = append(delegations, chainDelegation{
			DelegatorAddress: delegatorAddress.Hex(),
			Amount:           amountCopy,
		})
		total.Add(total, amountCopy)
	}
	return delegations, total, nil
}

func syncNodeDelegationsFromChainTx(tx *gorm.DB, nodeAddress, network string, delegations []chainDelegation) error {
	if err := tx.Model(&models.Delegation{}).
		Where("node_address = ?", nodeAddress).
		Where("network = ?", network).
		Update("valid", false).Error; err != nil {
		return err
	}
	for _, delegation := range delegations {
		row := models.Delegation{
			DelegatorAddress: delegation.DelegatorAddress,
			NodeAddress:      nodeAddress,
			Amount:           models.BigInt{Int: *delegation.Amount},
			Valid:            true,
			Network:          network,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "delegator_address"}, {Name: "node_address"}, {Name: "network"}},
			DoUpdates: clause.AssignmentColumns([]string{"amount", "valid", "updated_at"}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func applyNodeDelegationsToCache(nodeAddress, network string, delegations []chainDelegation) {
	RemoveNodeDelegations(nodeAddress, network)
	for _, delegation := range delegations {
		UpdateDelegation(delegation.DelegatorAddress, nodeAddress, delegation.Amount, network)
	}
}

func SetNodeStatusQuit(ctx context.Context, db *gorm.DB, node *models.Node, slashed bool) error {
	var wasActiveBeforeQuit bool
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		wasActiveBeforeQuit, err = setNodeStatusQuitTx(ctx, tx, node, slashed)
		return err
	})
	if err != nil {
		return err
	}
	applyNodeQuitPostCommit(node, wasActiveBeforeQuit)
	return nil
}

func setNodeStatusQuitTx(ctx context.Context, tx *gorm.DB, node *models.Node, slashed bool) (bool, error) {
	wasActiveBeforeQuit := IsNodeStatusActiveForNodeNameCount(node.Status)
	// delete all node local models
	err := tx.Where("node_address = ?", node.Address).Delete(&models.NodeModel{}).Error
	if err != nil {
		return false, err
	}

	if err := node.Update(ctx, tx, map[string]interface{}{
		"status":                     models.NodeStatusQuit,
		"current_task_id_commitment": sql.NullString{Valid: false},
		"stake_amount":               models.BigInt{Int: *big.NewInt(0)},
	}); err != nil {
		return false, err
	}
	if wasActiveBeforeQuit {
		if err := DecrementNodeNameCountTx(ctx, tx, node); err != nil {
			return false, err
		}
	}
	var txID uint
	stakingInfo, err := blockchain.GetStakingInfo(ctx, common.HexToAddress(node.Address), node.Network)
	if err != nil {
		return false, err
	}
	if stakingInfo.Status != 0 { // not unstaked
		if slashed {
			blockchainTransaction, err := blockchain.QueueSlashStaking(ctx, tx, common.HexToAddress(node.Address), node.Network)
			if err != nil {
				return false, err
			}
			txID = blockchainTransaction.ID
		} else {
			blockchainTransaction, err := blockchain.QueueUnstake(ctx, tx, common.HexToAddress(node.Address), node.Network)
			if err != nil {
				return false, err
			}
			txID = blockchainTransaction.ID
		}
	}
	if err := emitEvent(ctx, tx, &models.NodeQuitEvent{NodeAddress: node.Address, BlockchainTransactionID: txID, Network: node.Network}); err != nil {
		return false, err
	}
	return wasActiveBeforeQuit, nil
}

func applyNodeQuitPostCommit(node *models.Node, wasActiveBeforeQuit bool) {
	UpdateMaxStaking(node.Address, big.NewInt(0))
	if wasActiveBeforeQuit {
		ApplyNodeNameCountDeltaToCache(node.GPUName, node.GPUVram, BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion), -1)
	}
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

	// Kick out nodes that breach the configured permanent kickout conditions.
	if ShouldPermanentKickout(node) {
		task, err := models.GetTaskByIDCommitment(ctx, db, taskIDCommitment)
		if err != nil {
			return err
		}
		healthMetrics := calculateCurrentNodeHealthMetrics(node)
		var wasActiveBeforeQuit bool
		if err := db.Transaction(func(tx *gorm.DB) error {
			var err error
			wasActiveBeforeQuit, err = setNodeStatusQuitTx(ctx, tx, node, false)
			if err != nil {
				return err
			}
			return emitEvent(ctx, tx, &models.NodeKickedOutEvent{NodeAddress: node.Address, TaskIDCommitment: taskIDCommitment, Network: node.Network})
		}); err != nil {
			return err
		}
		applyNodeQuitPostCommit(node, wasActiveBeforeQuit)
		LogNodeStatusChange(node, "kickout")
		logNodeKickoutHealthEvent(node, task, healthMetrics)
		return nil
	}

	switch node.Status {
	case models.NodeStatusBusy:
		if err := node.Update(ctx, db, map[string]interface{}{
			"status":                     models.NodeStatusAvailable,
			"current_task_id_commitment": sql.NullString{Valid: false},
		}); err != nil {
			return err
		}
		return nil
	case models.NodeStatusPendingQuit:
		if err := SetNodeStatusQuit(ctx, db, node, false); err != nil {
			return err
		}
		LogNodeStatusChange(node, "quit")
		return nil
	case models.NodeStatusPendingPause:
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := node.Update(ctx, tx, map[string]interface{}{
				"status":                     models.NodeStatusPaused,
				"current_task_id_commitment": sql.NullString{Valid: false},
			}); err != nil {
				return err
			}
			return DecrementNodeNameCountTx(ctx, tx, node)
		}); err != nil {
			return err
		}
		ApplyNodeNameCountDeltaToCache(node.GPUName, node.GPUVram, BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion), -1)
		LogNodeStatusChange(node, "pause")
		return nil
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
	slashedAmount := node.StakeAmount
	var wasActiveBeforeQuit bool
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		wasActiveBeforeQuit, err = setNodeStatusQuitTx(ctx, tx, node, true)
		if err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.NodeSlashedEvent{NodeAddress: node.Address, TaskIDCommitment: taskIDCommitment, Amount: slashedAmount, Network: node.Network})
	}); err != nil {
		return err
	}
	applyNodeQuitPostCommit(node, wasActiveBeforeQuit)
	LogNodeStatusChange(node, "slashed")
	return nil
}

func updateNodeQosScore(ctx context.Context, db *gorm.DB, node *models.Node, qos uint64) error {
	qosScore, err := getNodeTaskQosScore(node, qos)
	if err != nil {
		return err
	}
	if err := node.Update(ctx, db, map[string]interface{}{
		"qos_score": qosScore,
	}); err != nil {
		return err
	}
	node.QOSScore = qosScore
	return nil
}
