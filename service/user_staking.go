package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"sync"
	"time"

	"gorm.io/gorm"
)

var globalDelegationCaches map[string]*delegationCache

type delegationCache struct {
	sync.RWMutex
	nodeDelegations map[string]map[string]*big.Int
	userDelegations map[string]map[string]*big.Int
	userStakeAmount map[string]*big.Int
	nodeStakeAmount map[string]*big.Int
}

func newDelegationCaches() map[string]*delegationCache {
	appConfig := config.GetConfig()
	caches := make(map[string]*delegationCache)
	for network := range appConfig.Blockchains {
		caches[network] = &delegationCache{
			nodeDelegations: make(map[string]map[string]*big.Int),
			userDelegations: make(map[string]map[string]*big.Int),
			userStakeAmount: make(map[string]*big.Int),
			nodeStakeAmount: make(map[string]*big.Int),
		}
	}
	return caches
}

func (c *delegationCache) update(delegatorAddress, nodeAddress string, amount *big.Int) {
	c.Lock()
	defer c.Unlock()

	oldAmount := big.NewInt(0)
	if delegations, ok := c.nodeDelegations[nodeAddress]; ok {
		if val, ok1 := delegations[delegatorAddress]; ok1 {
			oldAmount.Set(val)
		}
	} else {
		c.nodeDelegations[nodeAddress] = make(map[string]*big.Int)
	}
	if _, ok := c.userDelegations[delegatorAddress]; !ok {
		c.userDelegations[delegatorAddress] = make(map[string]*big.Int)
	}
	if _, ok := c.userStakeAmount[delegatorAddress]; !ok {
		c.userStakeAmount[delegatorAddress] = big.NewInt(0)
	}
	if _, ok := c.nodeStakeAmount[nodeAddress]; !ok {
		c.nodeStakeAmount[nodeAddress] = big.NewInt(0)
	}
	c.userStakeAmount[delegatorAddress].Sub(c.userStakeAmount[delegatorAddress], oldAmount)
	c.userStakeAmount[delegatorAddress].Add(c.userStakeAmount[delegatorAddress], amount)
	c.nodeStakeAmount[nodeAddress].Sub(c.nodeStakeAmount[nodeAddress], oldAmount)
	c.nodeStakeAmount[nodeAddress].Add(c.nodeStakeAmount[nodeAddress], amount)
	c.nodeDelegations[nodeAddress][delegatorAddress] = big.NewInt(0).Set(amount)
	c.userDelegations[delegatorAddress][nodeAddress] = big.NewInt(0).Set(amount)
}

func (c *delegationCache) unstake(delegatorAddress, nodeAddress string) {
	c.Lock()
	defer c.Unlock()

	oldAmount := big.NewInt(0)
	if delegations, ok := c.nodeDelegations[nodeAddress]; ok {
		if val, ok1 := delegations[delegatorAddress]; ok1 {
			oldAmount.Set(val)
			delete(c.nodeDelegations[nodeAddress], delegatorAddress)
		}
		if len(c.nodeDelegations[nodeAddress]) == 0 {
			delete(c.nodeDelegations, nodeAddress)
		}
	}
	if nodeStakings, ok := c.userDelegations[delegatorAddress]; ok {
		if _, ok1 := nodeStakings[nodeAddress]; ok1 {
			delete(c.userDelegations[delegatorAddress], nodeAddress)
		}
		if len(c.userDelegations[delegatorAddress]) == 0 {
			delete(c.userDelegations, delegatorAddress)
		}
	}
	c.userStakeAmount[delegatorAddress].Sub(c.userStakeAmount[delegatorAddress], oldAmount)
	c.nodeStakeAmount[nodeAddress].Sub(c.nodeStakeAmount[nodeAddress], oldAmount)
}

func (c *delegationCache) removeNode(nodeAddress string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.nodeDelegations[nodeAddress]; ok {
		for delegatorAddress, amount := range c.nodeDelegations[nodeAddress] {
			c.userStakeAmount[delegatorAddress].Sub(c.userStakeAmount[delegatorAddress], amount)
		}
		delete(c.nodeDelegations, nodeAddress)
		c.nodeStakeAmount[nodeAddress] = big.NewInt(0)
	}
}

func (c *delegationCache) getDelegatorTotalStakeAmount(delegatorAddress string) *big.Int {
	c.RLock()
	defer c.RUnlock()
	res := big.NewInt(0)
	if val, ok := c.userStakeAmount[delegatorAddress]; ok {
		res.Set(val)
	}
	return res
}

func (c *delegationCache) getNodeTotalStakeAmount(nodeAddress string) *big.Int {
	c.RLock()
	defer c.RUnlock()
	res := big.NewInt(0)
	if val, ok := c.nodeStakeAmount[nodeAddress]; ok {
		res.Set(val)
	}
	return res
}

func (c *delegationCache) getDelegationsOfNode(nodeAddress string) map[string]*big.Int {
	c.RLock()
	defer c.RUnlock()

	res := make(map[string]*big.Int)
	if userStakings, ok := c.nodeDelegations[nodeAddress]; ok {
		for delegatorAddress, amount := range userStakings {
			res[delegatorAddress] = big.NewInt(0).Set(amount)
		}
	}
	return res
}

func (c *delegationCache) getDelegationsOfDelegator(delegatorAddress string) map[string]*big.Int {
	c.RLock()
	defer c.RUnlock()

	res := make(map[string]*big.Int)
	if nodeStakings, ok := c.userDelegations[delegatorAddress]; ok {
		for nodeAddress, amount := range nodeStakings {
			res[nodeAddress] = big.NewInt(0).Set(amount)
		}
	}
	return res
}

func (c *delegationCache) getDelegatorCountOfNode(nodeAddress string) int {
	c.RLock()
	defer c.RUnlock()

	if userStakings, ok := c.nodeDelegations[nodeAddress]; ok {
		return len(userStakings)
	} else {
		return 0
	}
}

func (c *delegationCache) getDelegationCountOfDelegator(delegatorAddress string) int {
	c.RLock()
	defer c.RUnlock()

	if nodeStakings, ok := c.userDelegations[delegatorAddress]; ok {
		return len(nodeStakings)
	} else {
		return 0
	}
}

func InitDelegationCaches(ctx context.Context, db *gorm.DB) error {
	globalDelegationCaches = newDelegationCaches()

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var userStakings []models.Delegation
	if err := db.WithContext(dbCtx).Model(&models.Delegation{}).Where("valid = ?", true).Find(&userStakings).Error; err != nil {
		return err
	}

	for _, userStaking := range userStakings {
		UpdateDelegation(userStaking.DelegatorAddress, userStaking.NodeAddress, &userStaking.Amount.Int, userStaking.Network)
	}
	return nil
}

func UpdateDelegation(delegatorAddress, nodeAddress string, amount *big.Int, network string) {
	globalDelegationCaches[network].update(delegatorAddress, nodeAddress, amount)
}

func UnstakeDelegation(delegatorAddress, nodeAddress, network string) {
	globalDelegationCaches[network].unstake(delegatorAddress, nodeAddress)
}

func RemoveNodeDelegations(nodeAddress, network string) {
	globalDelegationCaches[network].removeNode(nodeAddress)
}

func GetDelegatorTotalStakeAmount(delegatorAddress, network string) *big.Int {
	return globalDelegationCaches[network].getDelegatorTotalStakeAmount(delegatorAddress)
}

func GetNodeTotalStakeAmount(nodeAddress, network string) *big.Int {
	return globalDelegationCaches[network].getNodeTotalStakeAmount(nodeAddress)
}

func GetDelegationsOfNode(nodeAddress, network string) map[string]*big.Int {
	return globalDelegationCaches[network].getDelegationsOfNode(nodeAddress)
}

func GetDelegationsOfDelegator(delegatorAddress, network string) map[string]*big.Int {
	return globalDelegationCaches[network].getDelegationsOfDelegator(delegatorAddress)
}

func GetDelegatorCountOfNode(nodeAddress, network string) int {
	return globalDelegationCaches[network].getDelegatorCountOfNode(nodeAddress)
}

func GetDelegationCountOfDelegator(delegatorAddress, network string) int {
	return globalDelegationCaches[network].getDelegationCountOfDelegator(delegatorAddress)
}
