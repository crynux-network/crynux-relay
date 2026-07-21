package service

import (
	"context"
	"crynux_relay/config"
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

func (c *nodeVestingStakeCache) addresses() []string {
	c.RLock()
	defer c.RUnlock()

	addresses := make([]string, 0, len(c.recordsByAddress))
	for address := range c.recordsByAddress {
		addresses = append(addresses, address)
	}
	return addresses
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

func lockedEmissionCoefficient() float64 {
	appConfig := config.GetConfig()
	if appConfig == nil {
		return 1
	}
	return *appConfig.StakingScore.LockedEmissionCoefficient
}

func GetNodeScoreLockedEmissionAmount(address string, now time.Time) *big.Int {
	lockedEmission := GetNodeLockedVestingAmount(address, now)
	coefficient := lockedEmissionCoefficient()
	if coefficient == 1 {
		return lockedEmission
	}
	scaled := new(big.Rat).SetFloat64(coefficient)
	result := new(big.Int).Mul(lockedEmission, scaled.Num())
	return result.Quo(result, scaled.Denom())
}

func GetNodeScoreStakeAmount(node models.Node, now time.Time) *big.Int {
	if node.Status == models.NodeStatusQuit {
		return big.NewInt(0)
	}

	total := big.NewInt(0).Set(&node.StakeAmount.Int)
	total.Add(total, GetNodeTotalStakeAmount(node.Address, node.Network))
	total.Add(total, GetNodeScoreLockedEmissionAmount(node.Address, now))
	total.Add(total, GetCachedRelayAccountBalance(node.Address))
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

	addressSet := make(map[string]struct{})
	if globalNodeVestingStakeCache != nil {
		for _, address := range globalNodeVestingStakeCache.addresses() {
			addressSet[address] = struct{}{}
		}
	}

	var activeAddresses []string
	if err := db.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Distinct("address").
		Where("status = ?", models.VestingStatusActive).
		Where("slashed = ?", false).
		Find(&activeAddresses).Error; err != nil {
		return err
	}
	for _, address := range activeAddresses {
		addressSet[address] = struct{}{}
	}

	var nodeAddresses []string
	if err := db.WithContext(dbCtx).
		Model(&models.Node{}).
		Where("status != ?", models.NodeStatusQuit).
		Pluck("address", &nodeAddresses).Error; err != nil {
		return err
	}
	for _, address := range nodeAddresses {
		addressSet[address] = struct{}{}
	}

	for address := range addressSet {
		if err := RefreshNodeScoreStake(ctx, db, address, now); err != nil {
			return err
		}
	}
	return nil
}
