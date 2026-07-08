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
	Address                            string            `json:"address" gorm:"index"`
	Network                            string            `json:"network" gorm:"index"`
	Status                             models.NodeStatus `json:"status" gorm:"index"`
	GPUName                            string            `json:"gpu_name" gorm:"index"`
	GPUVram                            uint64            `json:"gpu_vram" gorm:"index"`
	Version                            string            `json:"version"`
	InUseModelIDs                      []string          `json:"in_use_model_ids"`
	ModelIDs                           []string          `json:"model_ids"`
	StakingScore                       float64           `json:"staking_score"`
	QOSScore                           float64           `json:"qos_score"`
	ProbWeight                         float64           `json:"prob_weight"`
	OperatorStaking                    models.BigInt     `json:"operator_staking"`
	DelegatorStaking                   models.BigInt     `json:"delegator_staking"`
	LockedEmission                     models.BigInt     `json:"locked_emission"`
	DelegatorShare                     uint8             `json:"delegator_share"`
	DelegatorsNum                      int               `json:"delegators_num"`
	TotalOperatorEarnings              models.BigInt     `json:"total_operator_earnings"`
	TodayOperatorEarnings              models.BigInt     `json:"today_operator_earnings"`
	TotalDelegatorEarnings             models.BigInt     `json:"total_delegator_earnings"`
	TodayDelegatorEarnings             models.BigInt     `json:"today_delegator_earnings"`
	EstimatedUpcomingOperatorEmission  models.BigInt     `json:"estimated_upcoming_operator_emission"`
	EstimatedUpcomingDelegatorEmission models.BigInt     `json:"estimated_upcoming_delegator_emission"`
	EmissionWeekStart                  int64             `json:"emission_week_start"`
	EmissionWeekEnd                    int64             `json:"emission_week_end"`
	EstimateUpdatedAt                  int64             `json:"estimate_updated_at"`
	DelegationApr12m                   float64           `json:"delegation_apr_12m"`
	EstimatedNext10kDelegationApr      float64           `json:"estimated_next_10k_delegation_apr"`
	EstimatedNext100kDelegationApr     float64           `json:"estimated_next_100k_delegation_apr"`
	EstimatedNext1mDelegationApr       float64           `json:"estimated_next_1m_delegation_apr"`
	AprObservationDays                 uint32            `json:"apr_observation_days"`
	DelegationAprUpdatedAt             int64             `json:"delegation_apr_updated_at"`
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

	now := time.Now().UTC()
	selectingProb := service.CalculateNodeSelectingProb(*node, now)

	delegatorStaking := service.GetNodeTotalStakeAmount(node.Address, node.Network)
	lockedEmission := service.GetNodeLockedVestingAmount(node.Address, now)
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
	operatorEmissionEstimate := service.GetNodeOperatorEmissionEstimate(node.Address)
	delegatorEmissionEstimate := service.GetNodeDelegationEmissionEstimate(node.Address)
	return &Node{
		Address:                            node.Address,
		Network:                            node.Network,
		Status:                             node.Status,
		GPUName:                            node.GPUName,
		GPUVram:                            node.GPUVram,
		QOSScore:                           selectingProb.QOSScore,
		StakingScore:                       selectingProb.StakingScore,
		ProbWeight:                         selectingProb.ProbWeight,
		Version:                            nodeVersion,
		InUseModelIDs:                      inUseModelIDs,
		ModelIDs:                           modelIDs,
		OperatorStaking:                    node.StakeAmount,
		DelegatorStaking:                   models.BigInt{Int: *delegatorStaking},
		LockedEmission:                     models.BigInt{Int: *lockedEmission},
		DelegatorShare:                     node.DelegatorShare,
		DelegatorsNum:                      delegatorsNum,
		TotalOperatorEarnings:              models.BigInt{Int: *totalOperatorEarnings},
		TodayOperatorEarnings:              models.BigInt{Int: *todayOperatorEarnings},
		TotalDelegatorEarnings:             models.BigInt{Int: *totalDelegatorEarnings},
		TodayDelegatorEarnings:             models.BigInt{Int: *todayDelegatorEarnings},
		EstimatedUpcomingOperatorEmission:  models.BigInt{Int: *operatorEmissionEstimate.EstimatedEmission},
		EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *delegatorEmissionEstimate.EstimatedEmission},
		EmissionWeekStart:                  operatorEmissionEstimate.EmissionWeekStart,
		EmissionWeekEnd:                    operatorEmissionEstimate.EmissionWeekEnd,
		EstimateUpdatedAt:                  operatorEmissionEstimate.EstimateUpdatedAt,
	}, nil
}

func applyDelegationAPRSnapshot(nodeData *Node, snapshot *models.DelegatedStakingNodeListSnapshot) {
	if snapshot == nil {
		return
	}
	nodeData.DelegationApr12m = snapshot.DelegationApr12m
	nodeData.EstimatedNext10kDelegationApr = snapshot.EstimatedNext10kDelegationApr
	nodeData.EstimatedNext100kDelegationApr = snapshot.EstimatedNext100kDelegationApr
	nodeData.EstimatedNext1mDelegationApr = snapshot.EstimatedNext1mDelegationApr
	nodeData.AprObservationDays = snapshot.AprObservationDays
	if !snapshot.DelegationAprUpdatedAt.IsZero() {
		nodeData.DelegationAprUpdatedAt = snapshot.DelegationAprUpdatedAt.Unix()
	}
}

type NodeResponse struct {
	response.Response
	Data *Node `json:"data"`
}
