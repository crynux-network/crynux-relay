package staking

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"math/big"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetStakingInput struct {
	Address string `json:"address" path:"address" description:"address" validate:"required"`
}

type GetStakingOutput struct {
	response.Response
	Data models.BigInt `json:"data"`
}

func GetStaking(c *gin.Context, in *GetStakingInput) (*GetStakingOutput, error) {
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &GetStakingOutput{
				Data: models.BigInt{Int: *big.NewInt(0)},
			}, nil
		}
		return nil, response.NewExceptionResponse(err)
	}

	return &GetStakingOutput{
		Data: node.StakeAmount,
	}, nil
}

func GetUserStakingOfNode(c *gin.Context, in *GetStakingInput) (*GetStakingOutput, error) {
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &GetStakingOutput{
				Data: models.BigInt{Int: *big.NewInt(0)},
			}, nil
		}
		return nil, response.NewExceptionResponse(err)
	}
	amount := service.GetUserStakeAmountOfNode(in.Address, node.Network)
	return &GetStakingOutput{
		Data: models.BigInt{Int: *amount},
	}, nil
}
