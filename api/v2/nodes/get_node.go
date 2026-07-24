package nodes

import (
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"math/big"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetNodeInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
}

type GetNodeInputWithSignature struct {
	GetNodeInput
	Timestamp *int64 `json:"timestamp" query:"timestamp" description:"Signature timestamp"`
	Signature string `json:"signature" query:"signature" description:"Signature"`
}

func authorizeGetNode(c *gin.Context, input *GetNodeInputWithSignature) error {
	if middleware.GetUserAddress(c) != input.Address {
		return response.NewValidationErrorResponse("address", "Signer not allowed")
	}
	return nil
}

func GetNode(c *gin.Context, input *GetNodeInputWithSignature) (*NodeResponse, error) {
	if err := authorizeGetNode(c, input); err != nil {
		return nil, err
	}

	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), input.Address)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		operatorEmissionEstimate := service.GetNodeOperatorEmissionEstimate(input.Address)
		delegatorEmissionEstimate := service.GetNodeDelegationEmissionEstimate(input.Address)
		return &NodeResponse{
			Data: &Node{
				Address:                            input.Address,
				Status:                             models.NodeStatusQuit,
				GPUName:                            "",
				GPUVram:                            0,
				Version:                            "",
				InUseModelIDs:                      []string{},
				ModelIDs:                           []string{},
				StakingScore:                       0,
				QOSScore:                           0,
				ProbWeight:                         0,
				DelegatorStaking:                   models.BigInt{Int: *big.NewInt(0)},
				OperatorStaking:                    models.BigInt{Int: *big.NewInt(0)},
				LockedEmission:                     models.BigInt{Int: *big.NewInt(0)},
				RelayAccountBalance:                models.BigInt{Int: *big.NewInt(0)},
				DelegatorShare:                     0,
				DelegatorsNum:                      0,
				TotalOperatorEarnings:              models.BigInt{Int: *big.NewInt(0)},
				TodayOperatorEarnings:              models.BigInt{Int: *big.NewInt(0)},
				TotalDelegatorEarnings:             models.BigInt{Int: *big.NewInt(0)},
				TodayDelegatorEarnings:             models.BigInt{Int: *big.NewInt(0)},
				EstimatedUpcomingOperatorEmission:  models.BigInt{Int: *operatorEmissionEstimate.EstimatedEmission},
				EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *delegatorEmissionEstimate.EstimatedEmission},
				EmissionWeekStart:                  operatorEmissionEstimate.EmissionWeekStart,
				EmissionWeekEnd:                    operatorEmissionEstimate.EmissionWeekEnd,
				EstimateUpdatedAt:                  operatorEmissionEstimate.EstimateUpdatedAt,
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
