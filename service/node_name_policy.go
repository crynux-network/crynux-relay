package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidNodeNameGPUName    = errors.New("invalid node name gpu_name")
	ErrInvalidNodeNameVersion    = errors.New("invalid node name node_version")
	ErrNodeNameWhitelistExists   = errors.New("node name whitelist entry already exists")
	ErrNodeNameWhitelistMissing  = errors.New("node name whitelist entry not found")
	ErrNodeNameCountEntryMissing = errors.New("node name count entry not found")
	ErrNodeNameCountAlreadyZero  = errors.New("node name count is already zero")
)

type NodeNameWhitelistEntry struct {
	GPUName     string `json:"gpu_name"`
	GPUVram     uint64 `json:"gpu_vram"`
	NodeVersion string `json:"node_version"`
}

type NodeNameCountEntry struct {
	GPUName     string `json:"gpu_name"`
	GPUVram     uint64 `json:"gpu_vram"`
	NodeVersion string `json:"node_version"`
	ActiveCount uint64 `json:"active_count"`
}

var (
	nodeNameWhitelistCacheMu sync.RWMutex
	nodeNameWhitelistCache   map[string]struct{}
	nodeNameWhitelistLoaded  bool
)

var (
	nodeNameCountCacheMu sync.RWMutex
	nodeNameCountCache   map[string]uint64
	nodeNameCountLoaded  bool
)

func BuildNodeVersion(major, minor, patch uint64) string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func ParseNodeVersion(version string) (uint64, uint64, uint64, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, ErrInvalidNodeNameVersion
	}
	major, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, ErrInvalidNodeNameVersion
	}
	minor, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, ErrInvalidNodeNameVersion
	}
	patch, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return 0, 0, 0, ErrInvalidNodeNameVersion
	}
	return major, minor, patch, nil
}

func NormalizeNodeNameEntry(gpuName string, gpuVram uint64, nodeVersion string) (NodeNameWhitelistEntry, error) {
	normalizedGPUName := strings.TrimSpace(gpuName)
	if normalizedGPUName == "" {
		return NodeNameWhitelistEntry{}, ErrInvalidNodeNameGPUName
	}
	major, minor, patch, err := ParseNodeVersion(strings.TrimSpace(nodeVersion))
	if err != nil {
		return NodeNameWhitelistEntry{}, err
	}
	return NodeNameWhitelistEntry{
		GPUName:     normalizedGPUName,
		GPUVram:     gpuVram,
		NodeVersion: BuildNodeVersion(major, minor, patch),
	}, nil
}

func BuildNodeNameKey(gpuName string, gpuVram uint64, nodeVersion string) string {
	return fmt.Sprintf("%s|%d|%s", gpuName, gpuVram, nodeVersion)
}

func BuildNodeNameKeyFromNode(node *models.Node) string {
	return BuildNodeNameKey(node.GPUName, node.GPUVram, BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion))
}

func IsNodeStatusActiveForNodeNameCount(status models.NodeStatus) bool {
	return status != models.NodeStatusQuit && status != models.NodeStatusPaused
}

func AddNodeNameWhitelist(ctx context.Context, db *gorm.DB, gpuName string, gpuVram uint64, nodeVersion string) error {
	entry, err := NormalizeNodeNameEntry(gpuName, gpuVram, nodeVersion)
	if err != nil {
		return err
	}

	record := models.NodeNameWhitelist{
		GPUName:     entry.GPUName,
		GPUVram:     entry.GPUVram,
		NodeVersion: entry.NodeVersion,
	}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNodeNameWhitelistExists
	}
	return RefreshNodeNameWhitelistCache(ctx, db)
}

func DeleteNodeNameWhitelist(ctx context.Context, db *gorm.DB, gpuName string, gpuVram uint64, nodeVersion string) error {
	entry, err := NormalizeNodeNameEntry(gpuName, gpuVram, nodeVersion)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).
		Where("gpu_name = ? AND gpu_vram = ? AND node_version = ?", entry.GPUName, entry.GPUVram, entry.NodeVersion).
		Delete(&models.NodeNameWhitelist{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNodeNameWhitelistMissing
	}
	return RefreshNodeNameWhitelistCache(ctx, db)
}

func ListNodeNameWhitelist(ctx context.Context, db *gorm.DB) ([]NodeNameWhitelistEntry, error) {
	var records []models.NodeNameWhitelist
	if err := db.WithContext(ctx).Model(&models.NodeNameWhitelist{}).
		Order("gpu_name ASC").
		Order("gpu_vram ASC").
		Order("node_version ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	entries := make([]NodeNameWhitelistEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, NodeNameWhitelistEntry{
			GPUName:     record.GPUName,
			GPUVram:     record.GPUVram,
			NodeVersion: record.NodeVersion,
		})
	}
	return entries, nil
}

func ListNodeNameWhitelistFromCache(ctx context.Context, db *gorm.DB) ([]NodeNameWhitelistEntry, error) {
	if err := ensureNodeNameWhitelistCacheLoaded(ctx, db); err != nil {
		return nil, err
	}
	nodeNameWhitelistCacheMu.RLock()
	defer nodeNameWhitelistCacheMu.RUnlock()

	entries := make([]NodeNameWhitelistEntry, 0, len(nodeNameWhitelistCache))
	for key := range nodeNameWhitelistCache {
		gpuName, gpuVram, nodeVersion, ok := parseNodeNameKey(key)
		if !ok {
			continue
		}
		entries = append(entries, NodeNameWhitelistEntry{
			GPUName:     gpuName,
			GPUVram:     gpuVram,
			NodeVersion: nodeVersion,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].GPUName != entries[j].GPUName {
			return entries[i].GPUName < entries[j].GPUName
		}
		if entries[i].GPUVram != entries[j].GPUVram {
			return entries[i].GPUVram < entries[j].GPUVram
		}
		return entries[i].NodeVersion < entries[j].NodeVersion
	})
	return entries, nil
}

func IsNodeNameWhitelisted(ctx context.Context, db *gorm.DB, gpuName string, gpuVram uint64, nodeVersion string) (bool, error) {
	entry, err := NormalizeNodeNameEntry(gpuName, gpuVram, nodeVersion)
	if err != nil {
		return false, err
	}
	if err := ensureNodeNameWhitelistCacheLoaded(ctx, db); err != nil {
		return false, err
	}
	key := BuildNodeNameKey(entry.GPUName, entry.GPUVram, entry.NodeVersion)
	nodeNameWhitelistCacheMu.RLock()
	defer nodeNameWhitelistCacheMu.RUnlock()
	_, ok := nodeNameWhitelistCache[key]
	return ok, nil
}

func RefreshNodeNameWhitelistCache(ctx context.Context, db *gorm.DB) error {
	nodeNameWhitelistCacheMu.Lock()
	defer nodeNameWhitelistCacheMu.Unlock()
	return loadNodeNameWhitelistCacheLocked(ctx, db)
}

func ensureNodeNameWhitelistCacheLoaded(ctx context.Context, db *gorm.DB) error {
	nodeNameWhitelistCacheMu.RLock()
	if nodeNameWhitelistLoaded {
		nodeNameWhitelistCacheMu.RUnlock()
		return nil
	}
	nodeNameWhitelistCacheMu.RUnlock()

	nodeNameWhitelistCacheMu.Lock()
	defer nodeNameWhitelistCacheMu.Unlock()
	if nodeNameWhitelistLoaded {
		return nil
	}
	return loadNodeNameWhitelistCacheLocked(ctx, db)
}

func loadNodeNameWhitelistCacheLocked(ctx context.Context, db *gorm.DB) error {
	entries, err := ListNodeNameWhitelist(ctx, db)
	if err != nil {
		return err
	}
	cache := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		key := BuildNodeNameKey(entry.GPUName, entry.GPUVram, entry.NodeVersion)
		cache[key] = struct{}{}
	}
	nodeNameWhitelistCache = cache
	nodeNameWhitelistLoaded = true
	return nil
}

func ListNodeNameCounts(ctx context.Context, db *gorm.DB) ([]NodeNameCountEntry, error) {
	var records []models.NodeNameCount
	if err := db.WithContext(ctx).Model(&models.NodeNameCount{}).
		Order("gpu_name ASC").
		Order("gpu_vram ASC").
		Order("node_version ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	entries := make([]NodeNameCountEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, NodeNameCountEntry{
			GPUName:     record.GPUName,
			GPUVram:     record.GPUVram,
			NodeVersion: record.NodeVersion,
			ActiveCount: record.ActiveCount,
		})
	}
	return entries, nil
}

func ListNodeNameCountsFromCache(ctx context.Context, db *gorm.DB) ([]NodeNameCountEntry, error) {
	if err := ensureNodeNameCountCacheLoaded(ctx, db); err != nil {
		return nil, err
	}
	nodeNameCountCacheMu.RLock()
	defer nodeNameCountCacheMu.RUnlock()

	entries := make([]NodeNameCountEntry, 0, len(nodeNameCountCache))
	for key, count := range nodeNameCountCache {
		gpuName, gpuVram, nodeVersion, ok := parseNodeNameKey(key)
		if !ok {
			continue
		}
		entries = append(entries, NodeNameCountEntry{
			GPUName:     gpuName,
			GPUVram:     gpuVram,
			NodeVersion: nodeVersion,
			ActiveCount: count,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].GPUName != entries[j].GPUName {
			return entries[i].GPUName < entries[j].GPUName
		}
		if entries[i].GPUVram != entries[j].GPUVram {
			return entries[i].GPUVram < entries[j].GPUVram
		}
		return entries[i].NodeVersion < entries[j].NodeVersion
	})
	return entries, nil
}

func GetNodeNameActiveCount(ctx context.Context, db *gorm.DB, gpuName string, gpuVram uint64, nodeVersion string) (uint64, error) {
	entry, err := NormalizeNodeNameEntry(gpuName, gpuVram, nodeVersion)
	if err != nil {
		return 0, err
	}
	if err := ensureNodeNameCountCacheLoaded(ctx, db); err != nil {
		return 0, err
	}
	key := BuildNodeNameKey(entry.GPUName, entry.GPUVram, entry.NodeVersion)
	nodeNameCountCacheMu.RLock()
	defer nodeNameCountCacheMu.RUnlock()
	return nodeNameCountCache[key], nil
}

func RefreshNodeNameCountCache(ctx context.Context, db *gorm.DB) error {
	nodeNameCountCacheMu.Lock()
	defer nodeNameCountCacheMu.Unlock()
	return loadNodeNameCountCacheLocked(ctx, db)
}

func ensureNodeNameCountCacheLoaded(ctx context.Context, db *gorm.DB) error {
	nodeNameCountCacheMu.RLock()
	if nodeNameCountLoaded {
		nodeNameCountCacheMu.RUnlock()
		return nil
	}
	nodeNameCountCacheMu.RUnlock()

	nodeNameCountCacheMu.Lock()
	defer nodeNameCountCacheMu.Unlock()
	if nodeNameCountLoaded {
		return nil
	}
	return loadNodeNameCountCacheLocked(ctx, db)
}

func loadNodeNameCountCacheLocked(ctx context.Context, db *gorm.DB) error {
	entries, err := ListNodeNameCounts(ctx, db)
	if err != nil {
		return err
	}
	cache := make(map[string]uint64, len(entries))
	for _, entry := range entries {
		key := BuildNodeNameKey(entry.GPUName, entry.GPUVram, entry.NodeVersion)
		cache[key] = entry.ActiveCount
	}
	nodeNameCountCache = cache
	nodeNameCountLoaded = true
	return nil
}

func IncrementNodeNameCountTx(ctx context.Context, tx *gorm.DB, node *models.Node) error {
	entry := models.NodeNameCount{
		GPUName:     node.GPUName,
		GPUVram:     node.GPUVram,
		NodeVersion: BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion),
		ActiveCount: 1,
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "gpu_name"},
			{Name: "gpu_vram"},
			{Name: "node_version"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"active_count": gorm.Expr("active_count + 1"),
		}),
	}).Create(&entry).Error
}

func DecrementNodeNameCountTx(ctx context.Context, tx *gorm.DB, node *models.Node) error {
	nodeVersion := BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion)
	result := tx.WithContext(ctx).Model(&models.NodeNameCount{}).
		Where("gpu_name = ? AND gpu_vram = ? AND node_version = ?", node.GPUName, node.GPUVram, nodeVersion).
		Where("active_count > 0").
		Update("active_count", gorm.Expr("active_count - 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var entry models.NodeNameCount
		err := tx.WithContext(ctx).Model(&models.NodeNameCount{}).
			Where("gpu_name = ? AND gpu_vram = ? AND node_version = ?", node.GPUName, node.GPUVram, nodeVersion).
			First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNodeNameCountEntryMissing
		}
		if err != nil {
			return err
		}
		return ErrNodeNameCountAlreadyZero
	}
	return tx.WithContext(ctx).Where("gpu_name = ? AND gpu_vram = ? AND node_version = ? AND active_count = 0", node.GPUName, node.GPUVram, nodeVersion).
		Delete(&models.NodeNameCount{}).Error
}

func ApplyNodeNameCountDeltaToCache(gpuName string, gpuVram uint64, nodeVersion string, delta int64) {
	nodeNameCountCacheMu.Lock()
	defer nodeNameCountCacheMu.Unlock()
	if !nodeNameCountLoaded {
		return
	}
	key := BuildNodeNameKey(gpuName, gpuVram, nodeVersion)
	current := int64(nodeNameCountCache[key])
	next := current + delta
	if next <= 0 {
		delete(nodeNameCountCache, key)
		return
	}
	nodeNameCountCache[key] = uint64(next)
}

func resetNodeNamePolicyCacheForTest() {
	nodeNameWhitelistCacheMu.Lock()
	nodeNameWhitelistCache = nil
	nodeNameWhitelistLoaded = false
	nodeNameWhitelistCacheMu.Unlock()

	nodeNameCountCacheMu.Lock()
	nodeNameCountCache = nil
	nodeNameCountLoaded = false
	nodeNameCountCacheMu.Unlock()
}

func parseNodeNameKey(key string) (string, uint64, string, bool) {
	parts := strings.Split(key, "|")
	if len(parts) != 3 {
		return "", 0, "", false
	}
	vram, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0, "", false
	}
	return parts[0], vram, parts[2], true
}
