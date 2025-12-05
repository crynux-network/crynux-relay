package network

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type GetAllNodesDataParams struct {
	Page     int    `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize int    `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
}

type NetworkNodeData struct {
	Address   string   `json:"address"`
	CardModel string   `json:"card_model"`
	VRam      int      `json:"v_ram"`
	Balance   string `json:"balance"`
	Staking   string `json:"staking"`
	QOSScore  float64  `json:"qos_score"`
	StakingScore float64  `json:"staking_score"`
	ProbWeight   float64  `json:"prob_weight"`
}

type GetAllNodesDataResponse struct {
	response.Response
	Data []NetworkNodeData `json:"data"`
}

func GetAllNodeData(_ *gin.Context, in *GetAllNodesDataParams) (*GetAllNodesDataResponse, error) {
	page := 1
	if in.Page > 0 {
		page = in.Page
	}
	pageSize := 30
	if in.PageSize > 0 {
		pageSize = in.PageSize
	}
	offset := (page - 1) * pageSize
	limit := pageSize

	var allNodeData []models.NetworkNodeData
	if err := config.GetDB().Model(&models.NetworkNodeData{}).Order("id ASC").Limit(limit).Offset(offset).Find(&allNodeData).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	var data []NetworkNodeData
	for _, node := range allNodeData {
		stakingProb, qosProb, prob := service.CalculateSelectingProb(&node.Staking.Int, service.GetMaxStaking(), node.QoS, service.GetMaxQosScore())
		data = append(data, NetworkNodeData{
			Address:   node.Address,
			CardModel: node.CardModel,
			VRam:      node.VRam,
			Balance:   node.Balance.String(),
			Staking:   node.Staking.String(),
			QOSScore:  qosProb,
			StakingScore: stakingProb,
			ProbWeight: prob,
		})
	}
	return &GetAllNodesDataResponse{
		Data: data,
	}, nil
}
