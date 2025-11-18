package service

import (
	"context"
	"crynux_relay/models"
	"sync"

	"gorm.io/gorm"
)

var globalDelegatorShareCache *DelegatorShareCache

type DelegatorShareCache struct {
	sync.RWMutex
	delegatorShares map[string]uint8
}

func (c *DelegatorShareCache) set(nodeAddress string, share uint8) {
	c.Lock()
	defer c.Unlock()

	if share > 0 {
		c.delegatorShares[nodeAddress] = share
	} else {
		delete(c.delegatorShares, nodeAddress)
	}
}

func (c *DelegatorShareCache) get(nodeAddress string) uint8 {
	c.RLock()
	defer c.RUnlock()

	if share, ok := c.delegatorShares[nodeAddress]; ok {
		return share
	}
	return 0
}

func InitDelegatorShareCache(ctx context.Context, db *gorm.DB) error {
	globalDelegatorShareCache = &DelegatorShareCache{
		delegatorShares: make(map[string]uint8),
	}
	nodes, err := models.GetDelegatedNodes(ctx, db)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		globalDelegatorShareCache.set(node.Address, node.DelegatorShare)
	}
	return nil
}

func SetDelegatorShare(nodeAddress string, share uint8) {
	globalDelegatorShareCache.set(nodeAddress, share)
}

func GetDelegatorShare(nodeAddress string) uint8 {
	return globalDelegatorShareCache.get(nodeAddress)
}
