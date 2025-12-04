package delegator

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetDelegationsInput struct {
	UserAddress string `json:"user_address" path:"user_address" description:"address of the delegator" validate:"required"`
	Network     string `json:"network" query:"network" description:"network of the delegator" validate:"required"`
	Page        int    `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize    int    `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
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

type DelegationsResult struct {
	Delegations []DelegationInfo `json:"delegations"`
	Total       int64            `json:"total"`
}

type GetDelegationsOutput struct {
	response.Response
	Data *DelegationsResult `json:"data"`
}

func getDelegationsOfUser(ctx context.Context, db *gorm.DB, userAddress string, network *string, offset, limit int) ([]models.Delegation, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var userStakings []models.Delegation
	dbi := db.WithContext(dbCtx).Model(&models.Delegation{}).Where("delegator_address = ?", userAddress).Where("valid = ?", true)
	if network != nil {
		dbi = dbi.Where("network = ?", network)
	}

	var total int64
	if err := dbi.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := dbi.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&userStakings).Error; err != nil {
		return nil, 0, err
	}
	return userStakings, total, nil
}

func GetDelegations(c *gin.Context, input *GetDelegationsInput) (*GetDelegationsOutput, error) {
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
	userStakings, total, err := getDelegationsOfUser(c.Request.Context(), config.GetDB(), input.UserAddress, &input.Network, offset, limit)
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

	res := make([]DelegationInfo, 0)
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

	return &GetDelegationsOutput{
		Data: &DelegationsResult{
			Delegations: res,
			Total:       total,
		},
	}, nil
}
