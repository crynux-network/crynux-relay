package nodes

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"sort"

	"github.com/gin-gonic/gin"
)

type GetDelegatedNodesInput struct {
}

type GetDelegatedNodesOutput struct {
	response.Response
	Data []*Node `json:"data"`
}

func GetDelegatedNodes(c *gin.Context, input *GetDelegatedNodesInput) (*GetDelegatedNodesOutput, error) {
	nodes, err := models.GetDelegatedNodes(c.Request.Context(), config.GetDB())
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

	sort.Slice(nodeDatas, func(i, j int) bool {
		earningI := new(big.Int).Add(&nodeDatas[i].TodayOperatorEarnings.Int, &nodeDatas[i].TodayDelegatorEarnings.Int)
		earningJ := new(big.Int).Add(&nodeDatas[j].TodayOperatorEarnings.Int, &nodeDatas[j].TodayDelegatorEarnings.Int)
		return earningI.Cmp(earningJ) > 0
	})

	return &GetDelegatedNodesOutput{
		Data: nodeDatas,
	}, nil
}
