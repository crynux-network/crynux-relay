package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"math/big"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetNodeInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
}

func GetNode(c *gin.Context, input *GetNodeInput) (*NodeResponse, error) {
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), input.Address)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &NodeResponse{
			Data: &Node{
				Address:                input.Address,
				Status:                 models.NodeStatusQuit,
				GPUName:                "",
				GPUVram:                0,
				Version:                "",
				InUseModelIDs:          []string{},
				ModelIDs:               []string{},
				StakingScore:           0,
				QOSScore:               0,
				ProbWeight:             0,
				DelegatorStaking:       models.BigInt{Int: *big.NewInt(0)},
				OperatorStaking:        models.BigInt{Int: *big.NewInt(0)},
				DelegatorShare:         0,
				DelegatorsNum:          0,
				TotalOperatorEarnings:  models.BigInt{Int: *big.NewInt(0)},
				TodayOperatorEarnings:  models.BigInt{Int: *big.NewInt(0)},
				TotalDelegatorEarnings: models.BigInt{Int: *big.NewInt(0)},
				TodayDelegatorEarnings: models.BigInt{Int: *big.NewInt(0)},
			},
		}, nil
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	nodeData, err := getNodeData(c.Request.Context(), node)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &NodeResponse{
		Data: nodeData,
	}, nil
}
