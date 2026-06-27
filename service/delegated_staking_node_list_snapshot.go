package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const delegatedStakingNodeSnapshotBatchSize = 100

func BuildDelegatedStakingNodeStatusGroup(status models.NodeStatus) (string, uint8) {
	if status == models.NodeStatusQuit {
		return models.DelegatedStakingNodeStatusGroupStopped, 1
	}
	return models.DelegatedStakingNodeStatusGroupRunning, 0
}

func buildNodeVersion(major, minor, patch uint64) string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func BuildDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, node models.Node, now time.Time, mainnetStartTime string) (*models.DelegatedStakingNodeListSnapshot, error) {
	if node.DelegatorShare == 0 {
		return nil, nil
	}

	operatorEmission, err := getNodeOperatorEmission4w(ctx, db, node.Address, now, mainnetStartTime)
	if err != nil {
		return nil, err
	}

	lockedEmission := GetNodeLockedVestingAmount(node.Address, now)
	operatorStaking := big.NewInt(0).Add(&node.StakeAmount.Int, lockedEmission)
	delegatorStaking := GetNodeTotalStakeAmount(node.Address, node.Network)
	totalStaking := big.NewInt(0).Add(operatorStaking, delegatorStaking)
	delegatorsNum := GetDelegatorCountOfNode(node.Address, node.Network)
	selectingProb := CalculateNodeSelectingProb(node, now)
	statusGroup, statusRank := BuildDelegatedStakingNodeStatusGroup(node.Status)

	return &models.DelegatedStakingNodeListSnapshot{
		NodeAddress:        node.Address,
		Network:            node.Network,
		Status:             node.Status,
		StatusGroup:        statusGroup,
		StatusRank:         statusRank,
		GPUName:            node.GPUName,
		GPUVram:            node.GPUVram,
		Version:            buildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion),
		OperatorEmission4w: models.BigInt{Int: *operatorEmission},
		OperatorStaking:    models.BigInt{Int: *operatorStaking},
		DelegatorStaking:   models.BigInt{Int: *delegatorStaking},
		TotalStaking:       models.BigInt{Int: *totalStaking},
		DelegatorsNum:      uint64(delegatorsNum),
		ProbWeight:         selectingProb.ProbWeight,
		QOS:                selectingProb.QOSScore,
	}, nil
}

func getNodeOperatorEmission4w(ctx context.Context, db *gorm.DB, nodeAddress string, now time.Time, mainnetStartTime string) (*big.Int, error) {
	chartRange, err := BuildEmissionChartRange(now, mainnetStartTime, 4)
	if err != nil {
		return nil, err
	}
	records, err := models.ListVestingRecordsByAddressAndTypeAndStartTimeRange(ctx, db, nodeAddress, models.VestingTypeNode, chartRange.RangeStart, chartRange.RangeEnd)
	if err != nil {
		return nil, err
	}

	total := big.NewInt(0)
	for _, record := range records {
		total.Add(total, &record.TotalAmount.Int)
	}
	return total, nil
}

func RebuildDelegatedStakingNodeListSnapshots(ctx context.Context, db *gorm.DB, now time.Time, mainnetStartTime string) error {
	nodes, err := models.GetDelegatedNodes(ctx, db)
	if err != nil {
		return err
	}

	snapshots := make([]models.DelegatedStakingNodeListSnapshot, 0, len(nodes))
	for _, node := range nodes {
		snapshot, err := BuildDelegatedStakingNodeListSnapshot(ctx, db, *node, now, mainnetStartTime)
		if err != nil {
			return err
		}
		if snapshot != nil {
			snapshots = append(snapshots, *snapshot)
		}
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.DelegatedStakingNodeListSnapshot{}).Error; err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return nil
		}
		return tx.CreateInBatches(snapshots, delegatedStakingNodeSnapshotBatchSize).Error
	})
}

func RefreshDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	appConfig := config.GetConfig()
	if appConfig == nil {
		return nil
	}

	node, err := models.GetNodeByAddress(ctx, db, nodeAddress)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return deleteDelegatedStakingNodeListSnapshot(ctx, db, nodeAddress)
		}
		return err
	}
	if node.DelegatorShare == 0 {
		return deleteDelegatedStakingNodeListSnapshot(ctx, db, nodeAddress)
	}

	snapshot, err := BuildDelegatedStakingNodeListSnapshot(ctx, db, *node, time.Now().UTC(), appConfig.Dao.MainnetStartTime)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return deleteDelegatedStakingNodeListSnapshot(ctx, db, nodeAddress)
	}

	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_address"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"network",
			"status",
			"status_group",
			"status_rank",
			"gpu_name",
			"gpu_vram",
			"version",
			"operator_emission_4w",
			"operator_staking",
			"delegator_staking",
			"total_staking",
			"delegators_num",
			"prob_weight",
			"qos",
			"updated_at",
		}),
	}).Create(snapshot).Error
}

func deleteDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	return db.WithContext(ctx).Where("node_address = ?", nodeAddress).Delete(&models.DelegatedStakingNodeListSnapshot{}).Error
}
