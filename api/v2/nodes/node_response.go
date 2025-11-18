package nodes

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"
)

type Node struct {
	Address                string            `json:"address" gorm:"index"`
	Status                 models.NodeStatus `json:"status" gorm:"index"`
	GPUName                string            `json:"gpu_name" gorm:"index"`
	GPUVram                uint64            `json:"gpu_vram" gorm:"index"`
	Version                string            `json:"version"`
	InUseModelIDs          []string          `json:"in_use_model_ids"`
	ModelIDs               []string          `json:"model_ids"`
	StakingScore           float64           `json:"staking_score"`
	QOSScore               float64           `json:"qos_score"`
	ProbWeight             float64           `json:"prob_weight"`
	OperatorStaking        models.BigInt     `json:"operator_staking"`
	DelegatorStaking       models.BigInt     `json:"delegator_staking"`
	DelegatorShare         uint8             `json:"delegator_share"`
	DelegatorsNum          int               `json:"delegators_num"`
	TotalOperatorEarnings  models.BigInt     `json:"total_operator_earnings"`
	TodayOperatorEarnings  models.BigInt     `json:"today_operator_earnings"`
	TotalDelegatorEarnings models.BigInt     `json:"total_delegator_earnings"`
	TodayDelegatorEarnings models.BigInt     `json:"today_delegator_earnings"`
}

func getNodeData(ctx context.Context, node *models.Node) (*Node, error) {
	nodeModels, err := models.GetNodeModelsByNodeAddress(ctx, config.GetDB(), node.Address)
	if err != nil {
		return nil, err
	}

	modelIDs := make([]string, 0)
	inUseModelIDs := make([]string, 0)
	for _, model := range nodeModels {
		modelIDs = append(modelIDs, model.ModelID)
		if model.InUse {
			inUseModelIDs = append(inUseModelIDs, model.ModelID)
		}
	}

	nodeVersion := fmt.Sprintf("%d.%d.%d", node.MajorVersion, node.MinorVersion, node.PatchVersion)

	totalStakeAmount := big.NewInt(0)
	if node.Status != models.NodeStatusQuit {
		totalStakeAmount = new(big.Int).Add(&node.StakeAmount.Int, service.GetUserStakeAmountOfNode(node.Address, node.Network))
	}
	stakingScore, qosScore, probWeight := service.CalculateSelectingProb(totalStakeAmount, service.GetMaxStaking(), node.QOSScore, service.GetMaxQosScore())

	delegatorStaking := service.GetUserStakeAmountOfNode(node.Address, node.Network)
	delegatorsNum := service.GetDelegatorCountOfNode(node.Address, node.Network)

	totalOperatorEarnings := big.NewInt(0)
	totalDelegatorEarnings := big.NewInt(0)
	totalNodeEarning, err := models.GetTotalNodeEarning(ctx, config.GetDB(), node.Address)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		totalOperatorEarnings = &totalNodeEarning.OperatorEarning.Int
		totalDelegatorEarnings = &totalNodeEarning.DelegatorEarning.Int
	}

	todayOperatorEarnings := big.NewInt(0)
	todayDelegatorEarnings := big.NewInt(0)
	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	todayNodeEarnings, err := models.GetNodeEarnings(ctx, config.GetDB(), node.Address, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if len(todayNodeEarnings) > 0 {
		todayOperatorEarnings = &todayNodeEarnings[0].OperatorEarning.Int
		todayDelegatorEarnings = &todayNodeEarnings[0].DelegatorEarning.Int
	}
	return &Node{
		Address:                node.Address,
		Status:                 node.Status,
		GPUName:                node.GPUName,
		GPUVram:                node.GPUVram,
		QOSScore:               qosScore,
		StakingScore:           stakingScore,
		ProbWeight:             probWeight,
		Version:                nodeVersion,
		InUseModelIDs:          inUseModelIDs,
		ModelIDs:               modelIDs,
		OperatorStaking:        node.StakeAmount,
		DelegatorStaking:       models.BigInt{Int: *delegatorStaking},
		DelegatorShare:         node.DelegatorShare,
		DelegatorsNum:          delegatorsNum,
		TotalOperatorEarnings:  models.BigInt{Int: *totalOperatorEarnings},
		TodayOperatorEarnings:  models.BigInt{Int: *todayOperatorEarnings},
		TotalDelegatorEarnings: models.BigInt{Int: *totalDelegatorEarnings},
		TodayDelegatorEarnings: models.BigInt{Int: *todayDelegatorEarnings},
	}, nil
}

type NodeResponse struct {
	response.Response
	Data *Node `json:"data"`
}
