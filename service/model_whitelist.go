package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
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
	err := loadModelWhitelistCacheLocked(ctx, db)
	modelWhitelistCacheMu.Unlock()
	if err != nil {
		return err
	}
	return afterModelWhitelistCacheUpdated(ctx, db)
}

func RefreshModelWhitelistCache(ctx context.Context, db *gorm.DB) error {
	modelWhitelistCacheMu.Lock()
	err := loadModelWhitelistCacheLocked(ctx, db)
	modelWhitelistCacheMu.Unlock()
	if err != nil {
		return err
	}
	return afterModelWhitelistCacheUpdated(ctx, db)
}

func modelWhitelistEnforced() bool {
	cfg := config.GetConfig()
	if cfg == nil {
		return false
	}
	return cfg.Task.ModelWhitelistEnabled
}

func afterModelWhitelistCacheUpdated(ctx context.Context, db *gorm.DB) error {
	if !modelWhitelistEnforced() {
		return nil
	}
	if err := pruneNonWhitelistedModelInventory(ctx, db); err != nil {
		return err
	}
	return nil
}

// filterWhitelistedModelFiles keeps only catalog rows whose repository is in
// the model whitelist when enforcement is enabled.
func filterWhitelistedModelFiles(files []models.ModelFile) ([]models.ModelFile, error) {
	if !modelWhitelistEnforced() {
		return files, nil
	}
	kept := make([]models.ModelFile, 0, len(files))
	for _, file := range files {
		entry, err := file.Entry()
		if err != nil {
			return nil, err
		}
		ok, err := IsModelWhitelisted(entry.Repository)
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

// assertWhitelistedModelFiles reports ErrModelNotWhitelisted when any file
// repository is missing from the whitelist while enforcement is enabled.
func assertWhitelistedModelFiles(files []models.ModelFile) error {
	if !modelWhitelistEnforced() {
		return nil
	}
	for _, file := range files {
		entry, err := file.Entry()
		if err != nil {
			return err
		}
		ok, err := IsModelWhitelisted(entry.Repository)
		if err != nil {
			return err
		}
		if !ok {
			return ErrModelNotWhitelisted
		}
	}
	return nil
}

func modelFileRejectedByWhitelist(file models.ModelFile) (bool, error) {
	if !modelWhitelistEnforced() {
		return false, nil
	}
	entry, err := file.Entry()
	if err != nil {
		return false, err
	}
	ok, err := IsModelWhitelisted(entry.Repository)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

// pruneNonWhitelistedModelInventory removes on-disk inventory, in-memory models,
// and loaded-model projection rows that are not in the model whitelist.
func pruneNonWhitelistedModelInventory(ctx context.Context, db *gorm.DB) error {
	var catalog []models.ModelFile
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := db.WithContext(dbCtx).Find(&catalog).Error; err != nil {
		return err
	}

	banned := make(map[models.Hash32]struct{})
	bannedBytes := make([][]byte, 0)
	for _, file := range catalog {
		entry, err := file.Entry()
		if err != nil {
			return err
		}
		ok, err := IsModelWhitelisted(entry.Repository)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			return err
		}
		if _, exists := banned[hash]; exists {
			continue
		}
		banned[hash] = struct{}{}
		bannedBytes = append(bannedBytes, append([]byte(nil), file.ModelFileHash...))
	}

	affected := make(map[string]struct{})
	if len(bannedBytes) > 0 {
		var rows []models.NodeModel
		if err := db.WithContext(dbCtx).Where("model_file_hash IN ?", bannedBytes).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			affected[row.NodeAddress] = struct{}{}
		}
	}

	inMemoryRows, err := models.ListNodeInMemoryModels(ctx, db)
	if err != nil {
		return err
	}
	clearMemory := make([]string, 0)
	for i := range inMemoryRows {
		shouldClear, err := inMemoryModelHasNonWhitelistedFile(ctx, db, &inMemoryRows[i], banned)
		if err != nil {
			return err
		}
		if shouldClear {
			clearMemory = append(clearMemory, inMemoryRows[i].NodeAddress)
			affected[inMemoryRows[i].NodeAddress] = struct{}{}
		}
	}

	loadedModels, err := models.ListLoadedModels(ctx, db)
	if err != nil {
		return err
	}
	loadedIDs := make([]uint, 0)
	for _, loadedModel := range loadedModels {
		ok, err := IsModelWhitelisted(loadedModel.ModelID)
		if err != nil {
			return err
		}
		if !ok {
			loadedIDs = append(loadedIDs, loadedModel.ID)
		}
	}

	addresses := make([]string, 0, len(affected))
	for address := range affected {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)

	apply := func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if len(bannedBytes) > 0 {
				if err := tx.Where("model_file_hash IN ?", bannedBytes).Delete(&models.NodeModel{}).Error; err != nil {
					return err
				}
			}
			for _, address := range clearMemory {
				if err := models.ClearNodeInMemoryModel(ctx, tx, address); err != nil {
					return err
				}
			}
			for _, address := range addresses {
				if err := pruneSelectionsForCurrentInventory(ctx, tx, address); err != nil {
					return err
				}
			}
			if len(loadedIDs) > 0 {
				if err := tx.Where("id IN ?", loadedIDs).Delete(&models.LoadedModel{}).Error; err != nil {
					return err
				}
			}
			return nil
		})
	}

	if len(addresses) > 0 {
		if err := ExecuteNodeStateUpdate(ctx, db, addresses, apply); err != nil {
			return err
		}
	} else if err := apply(); err != nil {
		return err
	}

	invalidateLoadedModelViewCache()
	if len(bannedBytes) > 0 || len(clearMemory) > 0 || len(loadedIDs) > 0 {
		log.Infof(
			"ModelWhitelist: pruned non-whitelisted inventory banned_files=%d cleared_in_memory=%d deleted_loaded_models=%d affected_nodes=%d",
			len(bannedBytes), len(clearMemory), len(loadedIDs), len(addresses),
		)
	}
	return nil
}

func inMemoryModelHasNonWhitelistedFile(ctx context.Context, db *gorm.DB, row *models.NodeInMemoryModel, banned map[models.Hash32]struct{}) (bool, error) {
	hashes, err := row.FileHashes()
	if err != nil {
		return false, err
	}
	if len(hashes) == 0 {
		return false, nil
	}
	for _, hash := range hashes {
		if _, ok := banned[hash]; ok {
			return true, nil
		}
	}
	files, err := models.GetModelFiles(ctx, db, hashes)
	if err != nil {
		return false, err
	}
	if len(files) != len(hashes) {
		return true, nil
	}
	for _, file := range files {
		entry, err := file.Entry()
		if err != nil {
			return false, err
		}
		ok, err := IsModelWhitelisted(entry.Repository)
		if err != nil {
			return false, err
		}
		if !ok {
			return true, nil
		}
	}
	return false, nil
}

func pruneSelectionsForCurrentInventory(ctx context.Context, tx *gorm.DB, address string) error {
	existing, err := models.GetNodeModelsByNodeAddress(ctx, tx, address)
	if err != nil {
		return err
	}
	exactHeld := make(map[models.Hash32]struct{}, len(existing))
	hashes := make([]models.Hash32, 0, len(existing))
	for _, model := range existing {
		hash, err := models.Hash32FromBytes(model.ModelFileHash)
		if err != nil {
			return err
		}
		exactHeld[hash] = struct{}{}
		hashes = append(hashes, hash)
	}
	files, err := models.GetModelFiles(ctx, tx, hashes)
	if err != nil {
		return err
	}
	sourceHeld := make(map[models.Hash32]struct{}, len(files))
	for _, file := range files {
		sourceHash, err := models.Hash32FromBytes(file.ModelFileSourceHash)
		if err != nil {
			return err
		}
		sourceHeld[sourceHash] = struct{}{}
	}
	return pruneUncoveredSelections(ctx, tx, address, exactHeld, sourceHeld)
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

// taskModelNamesForWhitelist returns the normalized HuggingFace repository
// names that MUST all be present in the model whitelist for the task.
// For SD tasks this includes base_model and every components.*.repo that is a
// required model file. For LLM and SDFT it is the task ModelName.
func taskModelNamesForWhitelist(task *models.InferenceTask) ([]string, error) {
	if task.TaskType == models.TaskTypeSD {
		requirements, err := models.DeriveTaskModelRequirements(task.TaskArgs, task.TaskType)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]struct{}, len(requirements.Entries))
		names := make([]string, 0, len(requirements.Entries))
		for _, entry := range requirements.Entries {
			normalized, err := NormalizeModelWhitelistName(entry.Repository)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			names = append(names, normalized)
		}
		if len(names) == 0 {
			return nil, ErrInvalidModelWhitelistName
		}
		return names, nil
	}

	normalized, err := NormalizeModelWhitelistName(task.ModelName)
	if err != nil {
		return nil, err
	}
	return []string{normalized}, nil
}

// CheckTaskModelsWhitelisted reports whether every repository required by the
// task is in the in-memory allowlist. It returns ErrModelNotWhitelisted when
// any required repository is missing.
func CheckTaskModelsWhitelisted(task *models.InferenceTask) error {
	names, err := taskModelNamesForWhitelist(task)
	if err != nil {
		return err
	}
	for _, name := range names {
		ok, err := IsModelWhitelisted(name)
		if err != nil {
			return err
		}
		if !ok {
			return ErrModelNotWhitelisted
		}
	}
	return nil
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
