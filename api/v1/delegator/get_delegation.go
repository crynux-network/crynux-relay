package delegator

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetDelegationInput struct {
	UserAddress string `json:"user_address" path:"user_address" description:"address of the delegator" validate:"required"`
	NodeAddress string `json:"node_address" query:"node_address" description:"node address of the delegation" validate:"required"`
	Network     string `json:"network" query:"network" description:"network of the delegation" validate:"required"`
}

type GetDelegationOutput struct {
	response.Response
	Data *DelegationInfo `json:"data"`
}

func GetDelegation(c *gin.Context, input *GetDelegationInput) (*GetDelegationOutput, error) {
	userStaking, err := models.GetDelegation(c.Request.Context(), config.GetDB(), input.UserAddress, input.NodeAddress, input.Network)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}

	totalEarningAmount := big.NewInt(0)
	totalEarning, err := models.GetTotalUserStakingEarning(c.Request.Context(), config.GetDB(), input.UserAddress, input.NodeAddress, input.Network)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewExceptionResponse(err)
		}
	} else {
		totalEarningAmount.Set(&totalEarning.Earning.Int)
	}

	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	todayEarningAmount := big.NewInt(0)
	todayEarnings, err := models.GetUserStakingEarnings(c.Request.Context(), config.GetDB(), input.UserAddress, input.NodeAddress, input.Network, start, end)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if len(todayEarnings) > 0 {
		todayEarningAmount.Set(&todayEarnings[0].Earning.Int)
	}

	return &GetDelegationOutput{
		Data: &DelegationInfo{
			UserAddress:   userStaking.DelegatorAddress,
			NodeAddress:   userStaking.NodeAddress,
			Network:       userStaking.Network,
			StakingAmount: userStaking.Amount.String(),
			StakedAt:      userStaking.UpdatedAt.Unix(),
			TotalEarnings: models.BigInt{Int: *totalEarningAmount},
			TodayEarnings: models.BigInt{Int: *todayEarningAmount},
		},
	}, nil
}
