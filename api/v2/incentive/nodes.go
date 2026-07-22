package incentive

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const topNodeIncentiveSize = 10

type GetNodeIncentiveParams struct{}

type NodeIncentive struct {
	NodeAddress       string  `json:"node_address"`
	Incentive         float64 `json:"incentive"`
	TaskCount         int64   `json:"task_count"`
	CardModel         string  `json:"card_model"`
	VRam              int     `json:"vram"`
	Staking           string  `json:"staking"`
	QOSScore          float64 `json:"qos_score"`
	StakingScore      float64 `json:"staking_score"`
	ProbWeight        float64 `json:"prob_weight"`
	SDTaskCount       int64   `json:"sd_task_count"`
	LLMTaskCount      int64   `json:"llm_task_count"`
	SDFTLoraTaskCount int64   `json:"sd_ft_lora_task_count"`
}

type GetNodeIncentiveData struct {
	Nodes []NodeIncentive `json:"nodes"`
}

type GetNodeIncentiveOutput struct {
	Data *GetNodeIncentiveData `json:"data"`
}

func GetNodeIncentive(c *gin.Context, input *GetNodeIncentiveParams) (*GetNodeIncentiveOutput, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	end := now
	start := end.Add(-24 * time.Hour)

	rows, err := config.GetDB().WithContext(ctx).Model(&models.NodeIncentive{}).
		Select("node_address, SUM(incentive) as incentive, SUM(task_count) as task_count, SUM(sd_task_count) as sd_task_count, SUM(llm_task_count) as llm_task_count, SUM(sd_ft_lora_task_count) as sd_ft_lora_task_count").
		Where("time >= ?", start).
		Where("time < ?", end).
		Group("node_address").
		Order("incentive DESC").
		Offset(0).
		Limit(topNodeIncentiveSize).
		Rows()

	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	defer rows.Close()

	nodeIncentiveMap := make(map[string]NodeIncentive)
	var nodeAddresses []string
	for rows.Next() {
		var nodeAddress string
		var incentive float64
		var task_count int64
		var sd_task_count sql.NullInt64
		var llm_task_count sql.NullInt64
		var sd_ft_lora_task_count sql.NullInt64

		if err := rows.Scan(&nodeAddress, &incentive, &task_count, &sd_task_count, &llm_task_count, &sd_ft_lora_task_count); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
		nodeIncentive := NodeIncentive{
			NodeAddress: nodeAddress,
			Incentive:   incentive,
			TaskCount:   task_count,
		}
		if sd_task_count.Valid {
			nodeIncentive.SDTaskCount = sd_task_count.Int64
		}
		if llm_task_count.Valid {
			nodeIncentive.LLMTaskCount = llm_task_count.Int64
		}
		if sd_ft_lora_task_count.Valid {
			nodeIncentive.SDFTLoraTaskCount = sd_ft_lora_task_count.Int64
		}
		nodeAddresses = append(nodeAddresses, nodeAddress)
		nodeIncentiveMap[nodeAddress] = nodeIncentive
	}

	var nodes []models.NetworkNodeData
	if err := config.GetDB().WithContext(ctx).Model(&models.NetworkNodeData{}).Where("address IN (?)", nodeAddresses).Find(&nodes).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	nodeMap, err := getNodesByAddress(ctx, nodeAddresses)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	for _, node := range nodes {

		if nodeIncentive, ok := nodeIncentiveMap[node.Address]; ok {
			nodeModel, ok := nodeMap[node.Address]
			if !ok {
				continue
			}
			selectingProb := service.CalculateNodeSelectingProb(nodeModel, now)
			nodeIncentive.CardModel = node.CardModel
			nodeIncentive.VRam = node.VRam
			nodeIncentive.QOSScore = selectingProb.QOSScore
			nodeIncentive.Staking = selectingProb.ScoreStakeAmount.String()
			nodeIncentive.StakingScore = selectingProb.StakingScore
			nodeIncentive.ProbWeight = selectingProb.ProbWeight
			nodeIncentiveMap[node.Address] = nodeIncentive
		}
	}

	var nodeIncentives []NodeIncentive
	for _, address := range nodeAddresses {
		if nodeIncentive, ok := nodeIncentiveMap[address]; ok {
			nodeIncentives = append(nodeIncentives, nodeIncentive)
		}
	}

	return &GetNodeIncentiveOutput{
		Data: &GetNodeIncentiveData{
			Nodes: nodeIncentives,
		},
	}, nil
}

type GetAllNodeIncentiveParams struct {
	Period   TimeUnit `query:"period" json:"period" validate:"required" enum:"Day,Week,Month"`
	Page     int      `query:"page" json:"page" default:"1"`
	PageSize int      `query:"page_size" json:"page_size" default:"30"`
}

type GetAllNodeIncentiveParamsWithSignature struct {
	GetAllNodeIncentiveParams
	Timestamp int64  `query:"timestamp" json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `query:"signature" json:"signature" description:"Signature" validate:"required"`
}

type GetAllNodeIncentiveData struct {
	Nodes []NodeIncentive `json:"nodes"`
	Total int64           `json:"total"`
}

type GetAllNodeIncentiveOutput struct {
	Data *GetAllNodeIncentiveData `json:"data"`
}

func GetAllNodeIncentive(c *gin.Context, input *GetAllNodeIncentiveParamsWithSignature) (*GetAllNodeIncentiveOutput, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	match, address, err := validate.ValidateSignature(input.GetAllNodeIncentiveParams, input.Timestamp, input.Signature)
	if err != nil || !match {

		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}

		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	appConfig := config.GetConfig()
	var rootAddress string
	for _, blockchain := range appConfig.Blockchains {
		rootAddress = blockchain.Account.Address
		break
	}
	if address != rootAddress {
		validationErr := response.NewValidationErrorResponse("address", "Signer not allowed")
		return nil, validationErr
	}

	limit := input.PageSize
	offset := (input.Page - 1) * limit
	var start, end time.Time
	now := time.Now().UTC()
	switch input.Period {
	case UnitDay:
		duration := 24 * time.Hour
		end = now.Truncate(duration)
		start = end.Add(-duration)
	case UnitWeek:
		duration := 7 * 24 * time.Hour
		end = now.Truncate(duration)
		start = end.Add(-duration)
	default:
		year, month, _ := time.Now().UTC().Date()
		start = time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	}

	var total int64
	if err := config.GetDB().WithContext(ctx).Model(&models.NodeIncentive{}).
		Where("time >= ?", start).
		Where("time < ?", end).
		Count(&total).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	rows, err := config.GetDB().WithContext(ctx).Model(&models.NodeIncentive{}).
		Select("node_address, SUM(incentive) as incentive, SUM(task_count) as task_count, SUM(sd_task_count) as sd_task_count, SUM(llm_task_count) as llm_task_count, SUM(sd_ft_lora_task_count) as sd_ft_lora_task_count").
		Where("time >= ?", start).
		Where("time < ?", end).
		Group("node_address").
		Order("incentive DESC").
		Offset(offset).
		Limit(limit).
		Rows()

	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	defer rows.Close()

	nodeIncentiveMap := make(map[string]NodeIncentive)
	var nodeAddresses []string
	for rows.Next() {
		var nodeAddress string
		var incentive float64
		var task_count int64
		var sd_task_count sql.NullInt64
		var llm_task_count sql.NullInt64
		var sd_ft_lora_task_count sql.NullInt64

		if err := rows.Scan(&nodeAddress, &incentive, &task_count, &sd_task_count, &llm_task_count, &sd_ft_lora_task_count); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
		nodeIncentive := NodeIncentive{
			NodeAddress: nodeAddress,
			Incentive:   incentive,
			TaskCount:   task_count,
		}
		if sd_task_count.Valid {
			nodeIncentive.SDTaskCount = sd_task_count.Int64
		}
		if llm_task_count.Valid {
			nodeIncentive.LLMTaskCount = llm_task_count.Int64
		}
		if sd_ft_lora_task_count.Valid {
			nodeIncentive.SDFTLoraTaskCount = sd_ft_lora_task_count.Int64
		}
		nodeAddresses = append(nodeAddresses, nodeAddress)
		nodeIncentiveMap[nodeAddress] = nodeIncentive
	}

	var nodes []models.NetworkNodeData
	if err := config.GetDB().WithContext(ctx).Model(&models.NetworkNodeData{}).Where("address IN (?)", nodeAddresses).Find(&nodes).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	nodeMap, err := getNodesByAddress(ctx, nodeAddresses)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	for _, node := range nodes {

		if nodeIncentive, ok := nodeIncentiveMap[node.Address]; ok {
			nodeModel, ok := nodeMap[node.Address]
			if !ok {
				continue
			}
			selectingProb := service.CalculateNodeSelectingProb(nodeModel, now)
			nodeIncentive.CardModel = node.CardModel
			nodeIncentive.VRam = node.VRam
			nodeIncentive.QOSScore = selectingProb.QOSScore
			nodeIncentive.Staking = selectingProb.ScoreStakeAmount.String()
			nodeIncentive.StakingScore = selectingProb.StakingScore
			nodeIncentive.ProbWeight = selectingProb.ProbWeight

			nodeIncentiveMap[node.Address] = nodeIncentive
		}
	}

	var nodeIncentives []NodeIncentive
	for _, address := range nodeAddresses {
		if nodeIncentive, ok := nodeIncentiveMap[address]; ok {
			nodeIncentives = append(nodeIncentives, nodeIncentive)
		}
	}

	return &GetAllNodeIncentiveOutput{
		Data: &GetAllNodeIncentiveData{
			Nodes: nodeIncentives,
			Total: total,
		},
	}, nil
}

func getNodesByAddress(ctx context.Context, nodeAddresses []string) (map[string]models.Node, error) {
	var nodes []models.Node
	if err := config.GetDB().WithContext(ctx).Model(&models.Node{}).Where("address IN (?)", nodeAddresses).Find(&nodes).Error; err != nil {
		return nil, err
	}

	nodeMap := make(map[string]models.Node, len(nodes))
	for _, node := range nodes {
		nodeMap[node.Address] = node
	}
	return nodeMap, nil
}
