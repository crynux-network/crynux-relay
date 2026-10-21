package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"sort"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidModelWhitelistName    = errors.New("invalid model whitelist name")
	ErrModelWhitelistNameExists     = errors.New("model whitelist name already exists")
	ErrModelWhitelistNameMissing    = errors.New("model whitelist name not found")
	ErrModelWhitelistNotInitialized = errors.New("model whitelist cache is not initialized")
	ErrModelNotWhitelisted          = errors.New("Model not allowed")
)

var (
	modelWhitelistCacheMu sync.RWMutex
	modelWhitelistCache   map[string]struct{}
	modelWhitelistLoaded  bool
)

func NormalizeModelWhitelistName(name string) (string, error) {
	normalized := models.NormalizeModelName(strings.TrimSpace(name))
	if normalized == "" {
		return "", ErrInvalidModelWhitelistName
	}
	return normalized, nil
}

func InitModelWhitelistCache(ctx context.Context, db *gorm.DB) error {
	modelWhitelistCacheMu.Lock()
	defer modelWhitelistCacheMu.Unlock()
	return loadModelWhitelistCacheLocked(ctx, db)
}

func RefreshModelWhitelistCache(ctx context.Context, db *gorm.DB) error {
	modelWhitelistCacheMu.Lock()
	defer modelWhitelistCacheMu.Unlock()
	return loadModelWhitelistCacheLocked(ctx, db)
}

func AddModelWhitelistName(ctx context.Context, db *gorm.DB, modelName string) error {
	normalizedName, err := NormalizeModelWhitelistName(modelName)
	if err != nil {
		return err
	}

	record := models.ModelWhitelist{ModelName: normalizedName}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrModelWhitelistNameExists
	}

	if err := RefreshModelWhitelistCache(ctx, db); err != nil {
		return err
	}
	return nil
}

func DeleteModelWhitelistName(ctx context.Context, db *gorm.DB, modelName string) error {
	normalizedName, err := NormalizeModelWhitelistName(modelName)
	if err != nil {
		return err
	}

	result := db.WithContext(ctx).Where("model_name = ?", normalizedName).Delete(&models.ModelWhitelist{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrModelWhitelistNameMissing
	}

	if err := RefreshModelWhitelistCache(ctx, db); err != nil {
		return err
	}
	return nil
}

func ListModelWhitelistNames(ctx context.Context, db *gorm.DB) ([]string, error) {
	var records []models.ModelWhitelist
	if err := db.WithContext(ctx).
		Model(&models.ModelWhitelist{}).
		Order("model_name ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.ModelName)
	}
	return names, nil
}

// IsModelWhitelisted reports whether modelName is in the in-memory allowlist.
// It reads only memory and does not load from the database.
func IsModelWhitelisted(modelName string) (bool, error) {
	normalizedName, err := NormalizeModelWhitelistName(modelName)
	if err != nil {
		return false, err
	}

	modelWhitelistCacheMu.RLock()
	defer modelWhitelistCacheMu.RUnlock()
	if !modelWhitelistLoaded {
		return false, ErrModelWhitelistNotInitialized
	}
	_, ok := modelWhitelistCache[normalizedName]
	return ok, nil
}

func loadModelWhitelistCacheLocked(ctx context.Context, db *gorm.DB) error {
	names, err := ListModelWhitelistNames(ctx, db)
	if err != nil {
		return err
	}

	cache := make(map[string]struct{}, len(names))
	for _, name := range names {
		cache[name] = struct{}{}
	}

	modelWhitelistCache = cache
	modelWhitelistLoaded = true
	return nil
}

func resetModelWhitelistCacheForTest() {
	modelWhitelistCacheMu.Lock()
	defer modelWhitelistCacheMu.Unlock()
	modelWhitelistCache = nil
	modelWhitelistLoaded = false
}

func getModelWhitelistCacheSnapshotForTest() []string {
	modelWhitelistCacheMu.RLock()
	defer modelWhitelistCacheMu.RUnlock()

	names := make([]string, 0, len(modelWhitelistCache))
	for name := range modelWhitelistCache {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
