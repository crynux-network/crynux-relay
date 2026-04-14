package nodes

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetDelegatedNodesInput struct {
	Page     int `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize int `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
}

type DelegatedNodesResult struct {
	Nodes []*Node `json:"nodes"`
	Total int64   `json:"total"`
}

type GetDelegatedNodesOutput struct {
	response.Response
	Data DelegatedNodesResult `json:"data"`
}

func getDelegatedNodes(ctx context.Context, db *gorm.DB, offset, limit int) ([]*models.Node, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	dbi := db.WithContext(dbCtx).Model(&models.Node{}).Where("delegator_share > ?", 0)
	var total int64
	if err := dbi.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var nodes []*models.Node
	if err := dbi.Joins("left join node_earnings on node_earnings.node_address=nodes.address and time is NULL").
		Order("(CAST(node_earnings.operator_earning AS DECIMAL(65,0))+CAST(node_earnings.delegator_earning AS DECIMAL(65,0))) DESC").
		Offset(offset).
		Limit(limit).
		Find(&nodes).Error; err != nil {
		return nil, 0, err
	}
	return nodes, total, nil
}

func GetDelegatedNodes(c *gin.Context, input *GetDelegatedNodesInput) (*GetDelegatedNodesOutput, error) {
	page := 1
	if input.Page > 0 {
		page = input.Page
	}
	pageSize := 30
	if input.PageSize > 0 {
		pageSize = input.PageSize
	}
	offset := (page - 1) * pageSize
	limit := pageSize
	nodes, total, err := getDelegatedNodes(c.Request.Context(), config.GetDB(), offset, limit)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeDatas := make([]*Node, 0)
	errCh := make(chan error, len(nodes))
	semaphone := make(chan struct{}, 10)
	for _, node := range nodes {
		go func(n *models.Node) {
			semaphone <- struct{}{}
			defer func() {
				<-semaphone
			}()
			nodeData, err := getNodeData(c.Request.Context(), n)
			if err != nil {
				errCh <- err
				return
			}
			nodeDatas = append(nodeDatas, nodeData)
			errCh <- nil
		}(node)
	}
	for i := 0; i < len(nodes); i++ {
		if err := <-errCh; err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	return &GetDelegatedNodesOutput{
		Data: DelegatedNodesResult{
			Nodes: nodeDatas,
			Total: total,
		},
	}, nil
}
