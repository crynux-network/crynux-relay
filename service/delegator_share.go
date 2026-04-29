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

func makeDelegatorShareKey(nodeAddress, network string) string {
	return nodeAddress + ":" + network
}

func (c *DelegatorShareCache) set(nodeAddress, network string, share uint8) {
	c.Lock()
	defer c.Unlock()
	key := makeDelegatorShareKey(nodeAddress, network)

	if share > 0 {
		c.delegatorShares[key] = share
	} else {
		delete(c.delegatorShares, key)
	}
}

func (c *DelegatorShareCache) get(nodeAddress, network string) uint8 {
	c.RLock()
	defer c.RUnlock()
	key := makeDelegatorShareKey(nodeAddress, network)

	if share, ok := c.delegatorShares[key]; ok {
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
		globalDelegatorShareCache.set(node.Address, node.Network, node.DelegatorShare)
	}
	return nil
}

func SetDelegatorShare(nodeAddress, network string, share uint8) {
	globalDelegatorShareCache.set(nodeAddress, network, share)
}

func GetDelegatorShare(nodeAddress, network string) uint8 {
	return globalDelegatorShareCache.get(nodeAddress, network)
}
