package delegator

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetDelegationsInput struct {
	UserAddress string  `json:"user_address" path:"user_address" description:"address of the delegator" validate:"required"`
	Network     *string `json:"network" query:"network" description:"network of the delegator"`
	Page        int     `json:"page" query:"page" description:"The page" default:"1" validate:"min=1"`
	PageSize    int     `json:"page_size" query:"page_size" description:"The page size" default:"30" validate:"max=100,min=1"`
}

type DelegationInfo struct {
	UserAddress                  string        `json:"user_address"`
	NodeAddress                  string        `json:"node_address"`
	Network                      string        `json:"network"`
	NodeCurrentBlockchainNetwork string        `json:"node_current_blockchain_network"`
	Status                       string        `json:"status"`
	StakingAmount                string        `json:"staking_amount"`
	StakedAt                     int64         `json:"staked_at"`
	TotalEarnings                models.BigInt `json:"total_earnings"`
	TodayEarnings                models.BigInt `json:"today_earnings"`
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
	dbi := db.WithContext(dbCtx).Model(&models.Delegation{}).Where("delegator_address = ?", userAddress)
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

func delegationEarningsKey(nodeAddress, network string) string {
	return nodeAddress + "\x00" + network
}

func delegationStatus(delegation models.Delegation, node *models.Node) string {
	if delegation.Slashed {
		return "slashed"
	}
	if node != nil && node.Network == delegation.Network {
		return "active"
	}
	return "inactive"
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
	userStakings, total, err := getDelegationsOfUser(c.Request.Context(), config.GetDB(), input.UserAddress, input.Network, offset, limit)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	totalEarningsMap := make(map[string]models.BigInt)
	todayEarningsMap := make(map[string]models.BigInt)
	var mu sync.Mutex

	semaphore := make(chan struct{}, 10)
	errCh := make(chan error, len(userStakings))
	var wg sync.WaitGroup
	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	for _, userStaking := range userStakings {
		us := userStaking
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()

			totalEarningAmount := big.NewInt(0)
			totalEarning, err := models.GetTotalUserStakingEarning(c.Request.Context(), config.GetDB(), input.UserAddress, us.NodeAddress, us.Network)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					errCh <- err
					return
				}
			} else {
				totalEarningAmount.Set(&totalEarning.Earning.Int)
			}
			mu.Lock()
			totalEarningsMap[delegationEarningsKey(us.NodeAddress, us.Network)] = models.BigInt{Int: *totalEarningAmount}
			mu.Unlock()

			todayEarnings, err := models.GetUserStakingEarnings(c.Request.Context(), config.GetDB(), input.UserAddress, us.NodeAddress, us.Network, start, end)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			if len(todayEarnings) > 0 {
				todayEarningsMap[delegationEarningsKey(us.NodeAddress, us.Network)] = models.BigInt{Int: todayEarnings[0].Earning.Int}
			} else {
				todayEarningsMap[delegationEarningsKey(us.NodeAddress, us.Network)] = models.BigInt{Int: *big.NewInt(0)}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	nodeAddresses := make([]string, 0, len(userStakings))
	seenNodeAddresses := make(map[string]struct{})
	for _, userStaking := range userStakings {
		if _, ok := seenNodeAddresses[userStaking.NodeAddress]; ok {
			continue
		}
		seenNodeAddresses[userStaking.NodeAddress] = struct{}{}
		nodeAddresses = append(nodeAddresses, userStaking.NodeAddress)
	}
	nodeMap := make(map[string]*models.Node, len(nodeAddresses))
	if len(nodeAddresses) > 0 {
		nodes, err := models.GetNodesByAddresses(c.Request.Context(), config.GetDB(), nodeAddresses)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
		for _, node := range nodes {
			nodeMap[node.Address] = node
		}
	}

	res := make([]DelegationInfo, 0)
	for _, userStaking := range userStakings {
		key := delegationEarningsKey(userStaking.NodeAddress, userStaking.Network)
		node := nodeMap[userStaking.NodeAddress]
		nodeNetwork := ""
		if node != nil {
			nodeNetwork = node.Network
		}
		res = append(res, DelegationInfo{
			UserAddress:                  userStaking.DelegatorAddress,
			NodeAddress:                  userStaking.NodeAddress,
			Network:                      userStaking.Network,
			NodeCurrentBlockchainNetwork: nodeNetwork,
			Status:                       delegationStatus(userStaking, node),
			StakingAmount:                userStaking.Amount.Int.String(),
			StakedAt:                     userStaking.UpdatedAt.Unix(),
			TotalEarnings:                totalEarningsMap[key],
			TodayEarnings:                todayEarningsMap[key],
		})
	}

	return &GetDelegationsOutput{
		Data: &DelegationsResult{
			Delegations: res,
			Total:       total,
		},
	}, nil
}
