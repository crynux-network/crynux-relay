package nodes

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetDelegationsInput struct {
	Address  string `json:"address" path:"address" description:"node address" validate:"required"`
	Network  string `json:"network" query:"network" description:"network of the delegations" validate:"required"`
	Page     int    `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize int    `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
}

type DBDelegationResult struct {
	UserAddress   string        `json:"user_address"`
	NodeAddress   string        `json:"node_address"`
	Network       string        `json:"network"`
	StakingAmount models.BigInt `json:"staking_amount"`
	StakedAt      time.Time     `json:"staked_at"`
	TotalEarnings models.BigInt `json:"total_earnings"`
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

func getDelegationsOfNode(ctx context.Context, db *gorm.DB, nodeAddress string, network *string, offset, limit int) ([]DBDelegationResult, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	dbi := db.WithContext(dbCtx).Model(&models.Delegation{}).Where("delegations.node_address = ?", nodeAddress).Where("delegations.valid = ?", true)
	if network != nil {
		dbi = dbi.Where("delegations.network = ?", network)
	}

	var total int64
	if err := dbi.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var result []DBDelegationResult
	if err := dbi.Select("delegations.delegator_address as user_address, delegations.node_address as node_address, delegations.network as network, delegations.amount as staking_amount, delegations.updated_at as staked_at, user_staking_earnings.earning as total_earnings").
		Joins("left join user_staking_earnings on user_staking_earnings.user_address=delegations.delegator_address and user_staking_earnings.node_address=delegations.node_address and user_staking_earnings.time is NULL").
		Order("CAST(total_earnings AS DECIMAL(65,0)) DESC").
		Offset(offset).
		Limit(limit).
		Find(&result).Error; err != nil {
		return nil, 0, err
	}
	return result, total, nil
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

	userStakings, total, err := getDelegationsOfNode(c.Request.Context(), config.GetDB(), input.Address, &input.Network, offset, limit)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	todayEarningsMap := make(map[string]models.BigInt)

	semaphore := make(chan struct{}, 10)
	errCh := make(chan error, len(userStakings))

	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	for _, userStaking := range userStakings {
		go func(userAddress string) {
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()
			todayEarnings, err := models.GetUserStakingEarnings(c.Request.Context(), config.GetDB(), userAddress, input.Address, input.Network, start, end)
			if err != nil {
				errCh <- err
				return
			}
			if len(todayEarnings) > 0 {
				todayEarningsMap[userAddress] = models.BigInt{Int: todayEarnings[0].Earning.Int}
			} else {
				todayEarningsMap[userAddress] = models.BigInt{Int: *big.NewInt(0)}
			}
			errCh <- nil
		}(userStaking.UserAddress)
	}
	for i := 0; i < len(userStakings); i++ {
		if err := <-errCh; err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	res := make([]DelegationInfo, 0)
	for _, userStaking := range userStakings {
		res = append(res, DelegationInfo{
			UserAddress:   userStaking.UserAddress,
			NodeAddress:   userStaking.NodeAddress,
			Network:       userStaking.Network,
			StakingAmount: userStaking.StakingAmount.Int.String(),
			StakedAt:      userStaking.StakedAt.Unix(),
			TotalEarnings: userStaking.TotalEarnings,
			TodayEarnings: todayEarningsMap[userStaking.UserAddress],
		})
	}

	return &GetDelegationsOutput{
		Data: &DelegationsResult{
			Delegations: res,
			Total:       total,
		},
	}, nil
}
