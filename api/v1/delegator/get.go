package delegator

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

type GetDelegatorInput struct {
	UserAddress string `json:"user_address" path:"user_address" description:"address of the delegator" validate:"required"`
}

type DelegatorInfo struct {
	DelegationNum                       int           `json:"delegation_num"`
	TotalStakingAmount                  models.BigInt `json:"total_staking_amount"`
	TotalDelegationEarnings             models.BigInt `json:"total_delegation_earnings"`
	EstimatedUpcomingDelegationEmission models.BigInt `json:"estimated_upcoming_delegation_emission"`
	EmissionWeekStart                   int64         `json:"emission_week_start"`
	EmissionWeekEnd                     int64         `json:"emission_week_end"`
	EstimateUpdatedAt                   int64         `json:"estimate_updated_at"`
}

type GetDelegatorOutput struct {
	response.Response
	Data *DelegatorInfo `json:"data"`
}

func GetDelegatorInfo(c *gin.Context, input *GetDelegatorInput) (*GetDelegatorOutput, error) {
	appConfig := config.GetConfig()
	delegationNum := 0
	totalStakingAmount := big.NewInt(0)
	for network := range appConfig.Blockchains {
		delegationNum += service.GetDelegationCountOfDelegator(input.UserAddress, network)
		totalStakingAmount.Add(totalStakingAmount, service.GetDelegatorTotalStakeAmount(input.UserAddress, network))
	}

	totalDelegationEarnings := big.NewInt(0)
	userEarning, err := models.GetTotalUserEarning(c.Request.Context(), config.GetDB(), input.UserAddress)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewExceptionResponse(err)
		}
	} else {
		totalDelegationEarnings.Set(&userEarning.Earning.Int)
	}
	emissionEstimate := service.GetUserDelegationEmissionEstimate(input.UserAddress)

	return &GetDelegatorOutput{
		Data: &DelegatorInfo{
			DelegationNum:                       delegationNum,
			TotalStakingAmount:                  models.BigInt{Int: *totalStakingAmount},
			TotalDelegationEarnings:             models.BigInt{Int: *totalDelegationEarnings},
			EstimatedUpcomingDelegationEmission: models.BigInt{Int: *emissionEstimate.EstimatedEmission},
			EmissionWeekStart:                   emissionEstimate.EmissionWeekStart,
			EmissionWeekEnd:                     emissionEstimate.EmissionWeekEnd,
			EstimateUpdatedAt:                   emissionEstimate.EstimateUpdatedAt,
		},
	}, nil
}
