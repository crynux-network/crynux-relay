package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"math/big"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type GetNodeInput struct {
	Address string `json:"address" path:"address" description:"node address" validate:"required"`
}

type GetNodeInputWithSignature struct {
	GetNodeInput
	Timestamp int64  `json:"timestamp" query:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" query:"signature" description:"Signature" validate:"required"`
}

func GetNode(c *gin.Context, input *GetNodeInputWithSignature) (*NodeResponse, error) {
	match, address, err := validate.ValidateSignature(input.GetNodeInput, input.Timestamp, input.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}

		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	if address != input.Address {
		validationErr := response.NewValidationErrorResponse("address", "Signer not allowed")
		return nil, validationErr
	}

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
