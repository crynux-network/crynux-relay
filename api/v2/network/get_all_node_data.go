package network

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"time"

	"github.com/gin-gonic/gin"
)

type GetAllNodesDataParams struct {
	Page     int `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize int `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
}

type NetworkNodeData struct {
	Address      string  `json:"address"`
	CardModel    string  `json:"card_model"`
	VRam         int     `json:"v_ram"`
	Staking      string  `json:"staking"`
	QOSScore     float64 `json:"qos_score"`
	StakingScore float64 `json:"staking_score"`
	ProbWeight   float64 `json:"prob_weight"`
}

type GetAllNodesDataResponse struct {
	response.Response
	Data []NetworkNodeData `json:"data"`
}

func GetAllNodeData(c *gin.Context, in *GetAllNodesDataParams) (*GetAllNodesDataResponse, error) {
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

	nodeAddresses := make([]string, 0, len(allNodeData))
	for _, node := range allNodeData {
		nodeAddresses = append(nodeAddresses, node.Address)
	}
	var nodeModels []models.Node
	if err := config.GetDB().WithContext(c.Request.Context()).Model(&models.Node{}).Where("address IN (?)", nodeAddresses).Find(&nodeModels).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	nodeModelMap := make(map[string]models.Node, len(nodeModels))
	for _, node := range nodeModels {
		nodeModelMap[node.Address] = node
	}

	var data []NetworkNodeData
	now := time.Now().UTC()
	for _, node := range allNodeData {
		selectingProb := service.NodeSelectingProb{}
		if nodeModel, ok := nodeModelMap[node.Address]; ok {
			selectingProb = service.CalculateNodeSelectingProb(nodeModel, now)
		}
		data = append(data, NetworkNodeData{
			Address:      node.Address,
			CardModel:    node.CardModel,
			VRam:         node.VRam,
			Staking:      node.Staking.String(),
			QOSScore:     selectingProb.QOSScore,
			StakingScore: selectingProb.StakingScore,
			ProbWeight:   selectingProb.ProbWeight,
		})
	}
	return &GetAllNodesDataResponse{
		Data: data,
	}, nil
}
