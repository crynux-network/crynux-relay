package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidTaskWhitelistAddress = errors.New("invalid task whitelist address")
	ErrTaskWhitelistAddressExists  = errors.New("task whitelist address already exists")
	ErrTaskWhitelistAddressMissing = errors.New("task whitelist address not found")
)

var (
	taskWhitelistCacheMu sync.RWMutex
	taskWhitelistCache   map[string]struct{}
	taskWhitelistLoaded  bool
)

func NormalizeTaskWhitelistAddress(address string) (string, error) {
	if !common.IsHexAddress(address) {
		return "", ErrInvalidTaskWhitelistAddress
	}
	return common.HexToAddress(address).Hex(), nil
}

func AddTaskWhitelistAddress(ctx context.Context, db *gorm.DB, address string) error {
	normalizedAddress, err := NormalizeTaskWhitelistAddress(address)
	if err != nil {
		return err
	}

	record := models.TaskWhitelist{Address: normalizedAddress}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskWhitelistAddressExists
	}

	if err := RefreshTaskWhitelistCache(ctx, db); err != nil {
		return err
	}
	return nil
}

func DeleteTaskWhitelistAddress(ctx context.Context, db *gorm.DB, address string) error {
	normalizedAddress, err := NormalizeTaskWhitelistAddress(address)
	if err != nil {
		return err
	}

	result := db.WithContext(ctx).Where("address = ?", normalizedAddress).Delete(&models.TaskWhitelist{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskWhitelistAddressMissing
	}

	if err := RefreshTaskWhitelistCache(ctx, db); err != nil {
		return err
	}
	return nil
}

func ListTaskWhitelistAddresses(ctx context.Context, db *gorm.DB) ([]string, error) {
	var records []models.TaskWhitelist
	if err := db.WithContext(ctx).
		Model(&models.TaskWhitelist{}).
		Order("address ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	addresses := make([]string, 0, len(records))
	for _, record := range records {
		addresses = append(addresses, record.Address)
	}
	return addresses, nil
}

func IsTaskCreatorWhitelisted(ctx context.Context, db *gorm.DB, address string) (bool, error) {
	normalizedAddress, err := NormalizeTaskWhitelistAddress(address)
	if err != nil {
		return false, err
	}

	if err := ensureTaskWhitelistCacheLoaded(ctx, db); err != nil {
		return false, err
	}

	taskWhitelistCacheMu.RLock()
	defer taskWhitelistCacheMu.RUnlock()
	_, ok := taskWhitelistCache[normalizedAddress]
	return ok, nil
}

func RefreshTaskWhitelistCache(ctx context.Context, db *gorm.DB) error {
	taskWhitelistCacheMu.Lock()
	defer taskWhitelistCacheMu.Unlock()
	return loadTaskWhitelistCacheLocked(ctx, db)
}

func ensureTaskWhitelistCacheLoaded(ctx context.Context, db *gorm.DB) error {
	taskWhitelistCacheMu.RLock()
	if taskWhitelistLoaded {
		taskWhitelistCacheMu.RUnlock()
		return nil
	}
	taskWhitelistCacheMu.RUnlock()

	taskWhitelistCacheMu.Lock()
	defer taskWhitelistCacheMu.Unlock()
	if taskWhitelistLoaded {
		return nil
	}
	return loadTaskWhitelistCacheLocked(ctx, db)
}

func loadTaskWhitelistCacheLocked(ctx context.Context, db *gorm.DB) error {
	addresses, err := ListTaskWhitelistAddresses(ctx, db)
	if err != nil {
		return err
	}

	cache := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		cache[address] = struct{}{}
	}

	taskWhitelistCache = cache
	taskWhitelistLoaded = true
	return nil
}

func resetTaskWhitelistCacheForTest() {
	taskWhitelistCacheMu.Lock()
	defer taskWhitelistCacheMu.Unlock()
	taskWhitelistCache = nil
	taskWhitelistLoaded = false
}

func getTaskWhitelistCacheSnapshotForTest() []string {
	taskWhitelistCacheMu.RLock()
	defer taskWhitelistCacheMu.RUnlock()

	addresses := make([]string, 0, len(taskWhitelistCache))
	for address := range taskWhitelistCache {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)
	return addresses
}
