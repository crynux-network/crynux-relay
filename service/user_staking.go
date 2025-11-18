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

var globalUserStakingCaches map[string]*userStakingCache

type userStakingCache struct {
	sync.RWMutex
	nodeUserStakings map[string]map[string]*big.Int
	userNodeStakings map[string]map[string]*big.Int
	userStakeAmount  map[string]*big.Int
	nodeStakeAmount  map[string]*big.Int
}

func newUserStakingCaches() map[string]*userStakingCache {
	appConfig := config.GetConfig()
	caches := make(map[string]*userStakingCache)
	for network := range appConfig.Blockchains {
		caches[network] = &userStakingCache{
			nodeUserStakings: make(map[string]map[string]*big.Int),
			userNodeStakings: make(map[string]map[string]*big.Int),
			userStakeAmount:  make(map[string]*big.Int),
			nodeStakeAmount:  make(map[string]*big.Int),
		}
	}
	return caches
}

func (c *userStakingCache) update(userAddress, nodeAddress string, amount *big.Int) {
	c.Lock()
	defer c.Unlock()

	oldAmount := big.NewInt(0)
	if userStakings, ok := c.nodeUserStakings[nodeAddress]; ok {
		if val, ok1 := userStakings[userAddress]; ok1 {
			oldAmount.Set(val)
		}
	} else {
		c.nodeUserStakings[nodeAddress] = make(map[string]*big.Int)
	}
	if _, ok := c.userNodeStakings[userAddress]; !ok {
		c.userNodeStakings[userAddress] = make(map[string]*big.Int)
	}
	if _, ok := c.userStakeAmount[userAddress]; !ok {
		c.userStakeAmount[userAddress] = big.NewInt(0)
	}
	if _, ok := c.nodeStakeAmount[nodeAddress]; !ok {
		c.nodeStakeAmount[nodeAddress] = big.NewInt(0)
	}
	c.userStakeAmount[userAddress].Sub(c.userStakeAmount[userAddress], oldAmount)
	c.userStakeAmount[userAddress].Add(c.userStakeAmount[userAddress], amount)
	c.nodeStakeAmount[nodeAddress].Sub(c.nodeStakeAmount[nodeAddress], oldAmount)
	c.nodeStakeAmount[nodeAddress].Add(c.nodeStakeAmount[nodeAddress], amount)
	c.nodeUserStakings[nodeAddress][userAddress] = big.NewInt(0).Set(amount)
	c.userNodeStakings[userAddress][nodeAddress] = big.NewInt(0).Set(amount)
}

func (c *userStakingCache) unstake(userAddress, nodeAddress string) {
	c.Lock()
	defer c.Unlock()

	oldAmount := big.NewInt(0)
	if userStakings, ok := c.nodeUserStakings[nodeAddress]; ok {
		if val, ok1 := userStakings[userAddress]; ok1 {
			oldAmount.Set(val)
			delete(c.nodeUserStakings[nodeAddress], userAddress)
		}
		if len(c.nodeUserStakings[nodeAddress]) == 0 {
			delete(c.nodeUserStakings, nodeAddress)
		}
	}
	if nodeStakings, ok := c.userNodeStakings[userAddress]; ok {
		if _, ok1 := nodeStakings[nodeAddress]; ok1 {
			delete(c.userNodeStakings[userAddress], nodeAddress)
		}
		if len(c.userNodeStakings[userAddress]) == 0 {
			delete(c.userNodeStakings, userAddress)
		}
	}
	c.userStakeAmount[userAddress].Sub(c.userStakeAmount[userAddress], oldAmount)
	c.nodeStakeAmount[nodeAddress].Sub(c.nodeStakeAmount[nodeAddress], oldAmount)
}

func (c *userStakingCache) removeNode(nodeAddress string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.nodeUserStakings[nodeAddress]; ok {
		for userAddress, amount := range c.nodeUserStakings[nodeAddress] {
			c.userStakeAmount[userAddress].Sub(c.userStakeAmount[userAddress], amount)
		}
		delete(c.nodeUserStakings, nodeAddress)
		c.nodeStakeAmount[nodeAddress] = big.NewInt(0)
	}
}

func (c *userStakingCache) getUserStakeAmount(userAddress string) *big.Int {
	c.RLock()
	defer c.RUnlock()
	res := big.NewInt(0)
	if val, ok := c.userStakeAmount[userAddress]; ok {
		res.Set(val)
	}
	return res
}

func (c *userStakingCache) getNodeStakeAmount(nodeAddress string) *big.Int {
	c.RLock()
	defer c.RUnlock()
	res := big.NewInt(0)
	if val, ok := c.nodeStakeAmount[nodeAddress]; ok {
		res.Set(val)
	}
	return res
}

func (c *userStakingCache) getUserStakingsOfNode(nodeAddress string) map[string]*big.Int {
	c.RLock()
	defer c.RUnlock()

	res := make(map[string]*big.Int)
	if userStakings, ok := c.nodeUserStakings[nodeAddress]; ok {
		for userAddress, amount := range userStakings {
			res[userAddress] = big.NewInt(0).Set(amount)
		}
	}
	return res
}

func (c *userStakingCache) getUserStakingsOfUser(userAddress string) map[string]*big.Int {
	c.RLock()
	defer c.RUnlock()

	res := make(map[string]*big.Int)
	if nodeStakings, ok := c.userNodeStakings[userAddress]; ok {
		for nodeAddress, amount := range nodeStakings {
			res[nodeAddress] = big.NewInt(0).Set(amount)
		}
	}
	return res
}

func (c *userStakingCache) getDelegatorCountOfNode(nodeAddress string) int {
	c.RLock()
	defer c.RUnlock()

	if userStakings, ok := c.nodeUserStakings[nodeAddress]; ok {
		return len(userStakings)
	} else {
		return 0
	}
}

func (c *userStakingCache) getDelegationCountOfUser(userAddress string) int {
	c.RLock()
	defer c.RUnlock()

	if nodeStakings, ok := c.userNodeStakings[userAddress]; ok {
		return len(nodeStakings)
	} else {
		return 0
	}
}

func InitUserStakingCache(ctx context.Context, db *gorm.DB) error {
	globalUserStakingCaches = newUserStakingCaches()

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var userStakings []models.UserStaking
	if err := db.WithContext(dbCtx).Model(&models.UserStaking{}).Where("valid = ?", true).Find(&userStakings).Error; err != nil {
		return err
	}

	for _, userStaking := range userStakings {
		UpdateUserStaking(userStaking.UserAddress, userStaking.NodeAddress, &userStaking.Amount.Int, userStaking.Network)
	}
	return nil
}

func UpdateUserStaking(userAddress, nodeAddress string, amount *big.Int, network string) {
	globalUserStakingCaches[network].update(userAddress, nodeAddress, amount)
}

func UnstakeUserStaking(userAddress, nodeAddress, network string) {
	globalUserStakingCaches[network].unstake(userAddress, nodeAddress)
}

func RemoveNodeUserStaking(nodeAddress, network string) {
	globalUserStakingCaches[network].removeNode(nodeAddress)
}

func GetUserStakeAmountOfUser(userAddress, network string) *big.Int {
	return globalUserStakingCaches[network].getUserStakeAmount(userAddress)
}

func GetUserStakeAmountOfNode(nodeAddress, network string) *big.Int {
	return globalUserStakingCaches[network].getNodeStakeAmount(nodeAddress)
}

func GetUserStakingsOfNode(nodeAddress, network string) map[string]*big.Int {
	return globalUserStakingCaches[network].getUserStakingsOfNode(nodeAddress)
}

func GetUserStakingsOfUser(userAddress, network string) map[string]*big.Int {
	return globalUserStakingCaches[network].getUserStakingsOfUser(userAddress)
}

func GetDelegatorCountOfNode(nodeAddress, network string) int {
	return globalUserStakingCaches[network].getDelegatorCountOfNode(nodeAddress)
}

func GetDelegationCountOfUser(userAddress, network string) int {
	return globalUserStakingCaches[network].getDelegationCountOfUser(userAddress)
}
