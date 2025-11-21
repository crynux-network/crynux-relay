package delegator

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetDelegationsInput struct {
	UserAddress string `json:"user_address" path:"user_address" description:"address of the delegator" validate:"required"`
	Network     string `json:"network" query:"network" description:"network of the delegator" validate:"required"`
}

type DelegationInfo struct {
	UserAddress   string        `json:"user_address"`
	NodeAddress   string        `json:"node_address"`
	Network       string        `json:"network"`
	StakingAmount string        `json:"staking_amount"`
	StakedAt      int64         `json:"staked_at"`
	TotalEarnings models.BigInt `json:"total_earnings"`
	TodayEarnings models.BigInt `json:"today_earnings"`
}

type GetDelegationsOutput struct {
	response.Response
	Data []DelegationInfo `json:"data"`
}

func GetDelegations(c *gin.Context, input *GetDelegationsInput) (*GetDelegationsOutput, error) {
	userStakings, err := models.GetDelegationsOfUser(c.Request.Context(), config.GetDB(), input.UserAddress, &input.Network)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	totalEarningsMap := make(map[string]models.BigInt)
	todayEarningsMap := make(map[string]models.BigInt)

	semaphore := make(chan struct{}, 10)
	errCh := make(chan error, len(userStakings))

	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	for _, userStaking := range userStakings {
		go func(nodeAddress string) {
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()
			totalEarningAmount := big.NewInt(0)
			totalEarning, err := models.GetTotalUserStakingEarning(c.Request.Context(), config.GetDB(), input.UserAddress, nodeAddress, input.Network)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					errCh <- err
					return
				}
			} else {
				totalEarningAmount.Set(&totalEarning.Earning.Int)
			}
			totalEarningsMap[nodeAddress] = models.BigInt{Int: *totalEarningAmount}

			todayEarnings, err := models.GetUserStakingEarnings(c.Request.Context(), config.GetDB(), input.UserAddress, nodeAddress, input.Network, start, end)
			if err != nil {
				errCh <- err
				return
			}
			if len(todayEarnings) > 0 {
				todayEarningsMap[nodeAddress] = models.BigInt{Int: todayEarnings[0].Earning.Int}
			} else {
				todayEarningsMap[nodeAddress] = models.BigInt{Int: *big.NewInt(0)}
			}
			errCh <- nil
		}(userStaking.NodeAddress)
	}
	for i := 0; i < len(userStakings); i++ {
		if err := <-errCh; err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	var res []DelegationInfo
	for _, userStaking := range userStakings {
		res = append(res, DelegationInfo{
			UserAddress:   userStaking.DelegatorAddress,
			NodeAddress:   userStaking.NodeAddress,
			Network:       userStaking.Network,
			StakingAmount: userStaking.Amount.Int.String(),
			StakedAt:      userStaking.UpdatedAt.Unix(),
			TotalEarnings: totalEarningsMap[userStaking.NodeAddress],
			TodayEarnings: todayEarningsMap[userStaking.NodeAddress],
		})
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].TotalEarnings.Cmp(&res[j].TotalEarnings.Int) > 0
	})

	return &GetDelegationsOutput{
		Data: res,
	}, nil
}
