package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"math/big"
	"sync"
	"time"

	"gorm.io/gorm"
)

const initNodeVestingStakeCacheBatchSize = 1000

var globalNodeVestingStakeCache *nodeVestingStakeCache

type nodeVestingStakeCache struct {
	sync.RWMutex
	recordsByAddress map[string][]models.VestingRecord
}

func newNodeVestingStakeCache() *nodeVestingStakeCache {
	return &nodeVestingStakeCache{
		recordsByAddress: make(map[string][]models.VestingRecord),
	}
}

func (c *nodeVestingStakeCache) set(address string, records []models.VestingRecord) {
	c.Lock()
	defer c.Unlock()

	if len(records) == 0 {
		delete(c.recordsByAddress, address)
		return
	}
	copied := make([]models.VestingRecord, len(records))
	copy(copied, records)
	c.recordsByAddress[address] = copied
}

func (c *nodeVestingStakeCache) lockedAmount(address string, now time.Time) *big.Int {
	c.RLock()
	defer c.RUnlock()

	total := big.NewInt(0)
	for _, record := range c.recordsByAddress[address] {
		total.Add(total, record.LockedAmountAt(now))
	}
	return total
}

func InitNodeVestingStakeCache(ctx context.Context, db *gorm.DB) error {
	cache := newNodeVestingStakeCache()

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var records []models.VestingRecord
	if err := db.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Where("status = ?", models.VestingStatusActive).
		Where("slashed = ?", false).
		Where("type = ?", models.VestingTypeNode).
		FindInBatches(&records, initNodeVestingStakeCacheBatchSize, func(tx *gorm.DB, batch int) error {
			for _, record := range records {
				addressRecords := cache.recordsByAddress[record.Address]
				cache.recordsByAddress[record.Address] = append(addressRecords, record)
			}
			return nil
		}).Error; err != nil {
		return err
	}

	globalNodeVestingStakeCache = cache
	return nil
}

func RefreshNodeVestingStake(ctx context.Context, db *gorm.DB, address string) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var records []models.VestingRecord
	if err := db.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Where("address = ?", address).
		Where("status = ?", models.VestingStatusActive).
		Where("slashed = ?", false).
		Where("type = ?", models.VestingTypeNode).
		Find(&records).Error; err != nil {
		return err
	}

	if globalNodeVestingStakeCache == nil {
		globalNodeVestingStakeCache = newNodeVestingStakeCache()
	}
	globalNodeVestingStakeCache.set(address, records)
	return nil
}

func GetNodeLockedVestingAmount(address string, now time.Time) *big.Int {
	if globalNodeVestingStakeCache == nil {
		return big.NewInt(0)
	}
	return globalNodeVestingStakeCache.lockedAmount(address, now)
}

func GetNodeScoreStakeAmount(node models.Node, now time.Time) *big.Int {
	if node.Status == models.NodeStatusQuit {
		return big.NewInt(0)
	}

	total := big.NewInt(0).Set(&node.StakeAmount.Int)
	total.Add(total, GetNodeTotalStakeAmount(node.Address, node.Network))
	total.Add(total, GetNodeLockedVestingAmount(node.Address, now))
	return total
}

func RefreshNodeScoreStake(ctx context.Context, db *gorm.DB, address string, now time.Time) error {
	if err := RefreshNodeVestingStake(ctx, db, address); err != nil {
		return err
	}

	node, err := models.GetNodeByAddress(ctx, db, address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	UpdateMaxStaking(address, GetNodeScoreStakeAmount(*node, now))
	return nil
}

func RefreshNodeVestingScoreStakes(ctx context.Context, db *gorm.DB, now time.Time) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var addresses []string
	if err := db.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Distinct("address").
		Where("status = ?", models.VestingStatusActive).
		Where("slashed = ?", false).
		Where("type = ?", models.VestingTypeNode).
		Find(&addresses).Error; err != nil {
		return err
	}

	for _, address := range addresses {
		if err := RefreshNodeScoreStake(ctx, db, address, now); err != nil {
			return err
		}
	}
	return nil
}
