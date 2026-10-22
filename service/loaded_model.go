package service

import (
	"context"
	"crynux_relay/models"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const loadedModelFlushInterval = time.Hour
const loadedModelNodeCountTTL = time.Minute

var loadedModelCache = newLoadedModelMinVRAMCache()
var loadedModelViewCache = &loadedModelViewCacheStore{}

// loadedModelKey is the identity of a loaded model row.
type loadedModelKey struct {
	RequiredModelFilesHash models.Hash32
	ModelCacheHash         models.Hash32
	ExecutionDType         string
}

// LoadedModelView is a loaded model together with the number of nodes that
// hold its model files on disk and in memory.
type LoadedModelView struct {
	Model             models.LoadedModel
	OnDiskNodeCount   int64
	InMemoryNodeCount int64
}

type loadedModelViewCacheStore struct {
	mu        sync.Mutex
	views     []LoadedModelView
	loaded    bool
	expiresAt time.Time
}

// GetLoadedModelViews returns the loaded models with node counts, cached in
// memory so that public API traffic does not translate into database load.
func GetLoadedModelViews(ctx context.Context, db *gorm.DB) ([]LoadedModelView, error) {
	return loadedModelViewCache.get(ctx, db, time.Now())
}

func (cache *loadedModelViewCacheStore) get(ctx context.Context, db *gorm.DB, now time.Time) ([]LoadedModelView, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.loaded && now.Before(cache.expiresAt) {
		return cache.views, nil
	}
	views, err := buildLoadedModelViews(ctx, db)
	if err != nil {
		return nil, err
	}
	cache.views = views
	cache.loaded = true
	cache.expiresAt = now.Add(loadedModelNodeCountTTL)
	return views, nil
}

func invalidateLoadedModelViewCache() {
	loadedModelViewCache.mu.Lock()
	defer loadedModelViewCache.mu.Unlock()
	loadedModelViewCache.views = nil
	loadedModelViewCache.loaded = false
	loadedModelViewCache.expiresAt = time.Time{}
}

func buildLoadedModelViews(ctx context.Context, db *gorm.DB) ([]LoadedModelView, error) {
	loadedModels, err := models.ListLoadedModels(ctx, db)
	if err != nil {
		return nil, err
	}
	state, err := loadNodeModelState(ctx, db)
	if err != nil {
		return nil, err
	}
	onDisk := make(map[string]map[models.Hash32]struct{}, len(state.files))
	for address, rows := range state.files {
		hashes := make(map[models.Hash32]struct{}, len(rows))
		for _, row := range rows {
			hash, err := models.Hash32FromBytes(row.ModelFileHash)
			if err != nil {
				return nil, err
			}
			hashes[hash] = struct{}{}
		}
		onDisk[address] = hashes
	}

	views := make([]LoadedModelView, 0, len(loadedModels))
	for _, loadedModel := range loadedModels {
		if modelWhitelistEnforced() {
			ok, err := IsModelWhitelisted(loadedModel.ModelID)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		members, err := loadedModel.MemberHashes()
		if err != nil {
			return nil, err
		}
		cacheHash, err := models.Hash32FromBytes(loadedModel.ModelCacheHash)
		if err != nil {
			return nil, err
		}
		view := LoadedModelView{Model: loadedModel}
		for _, hashes := range onDisk {
			if modelFilesCovered(hashes, nil, members, nil) {
				view.OnDiskNodeCount++
			}
		}
		for _, memory := range state.inMemory {
			if memory.ExecutionDType != loadedModel.ExecutionDType {
				continue
			}
			memoryCacheHash, err := models.Hash32FromBytes(memory.CurrentModelCacheHash)
			if err != nil {
				return nil, err
			}
			if memoryCacheHash != cacheHash {
				continue
			}
			memoryFiles, err := memory.FileHashes()
			if err != nil {
				return nil, err
			}
			if modelFilesCovered(hashSet(memoryFiles), nil, members, nil) {
				view.InMemoryNodeCount++
			}
		}
		views = append(views, view)
	}
	return views, nil
}

type pendingLoadedModel struct {
	ModelID                 string
	Variant                 string
	ModelType               models.LoadedModelType
	RequiredModelFileHashes []models.Hash32
	QuantizationConfig      []byte
	MinVRAM                 uint64
}

type loadedModelMinVRAMCache struct {
	mu      sync.Mutex
	pending map[loadedModelKey]pendingLoadedModel
}

func newLoadedModelMinVRAMCache() *loadedModelMinVRAMCache {
	return &loadedModelMinVRAMCache{
		pending: make(map[loadedModelKey]pendingLoadedModel),
	}
}

// updateLoadedModels records the successful execution of a task as a loaded model.
func updateLoadedModels(task *models.InferenceTask, node *models.Node) {
	if modelWhitelistEnforced() {
		ok, err := IsModelWhitelisted(task.ModelName)
		if err != nil {
			log.Errorf("UpdateLoadedModels: whitelist check of task %s error: %v", task.TaskIDCommitment, err)
			return
		}
		if !ok {
			return
		}
	}
	executionHashes, err := task.ExecutionModelFileHashList()
	if err != nil {
		log.Errorf("UpdateLoadedModels: decode execution model files of task %s error: %v", task.TaskIDCommitment, err)
		return
	}
	if len(executionHashes) == 0 {
		return
	}
	cacheHash, err := task.RequestedModelCacheHash32()
	if err != nil {
		log.Errorf("UpdateLoadedModels: decode model cache hash of task %s error: %v", task.TaskIDCommitment, err)
		return
	}
	quantizationConfig, err := models.TaskQuantizationConfigJSON(task.TaskArgs)
	if err != nil {
		log.Errorf("UpdateLoadedModels: decode quantization config of task %s error: %v", task.TaskIDCommitment, err)
		return
	}
	key := loadedModelKey{
		RequiredModelFilesHash: models.RequiredModelFilesHash(executionHashes),
		ModelCacheHash:         cacheHash,
		ExecutionDType:         task.ExecutionDType,
	}
	loadedModelCache.record(key, pendingLoadedModel{
		ModelID:                 task.ModelName,
		Variant:                 task.ModelVariant,
		ModelType:               models.LoadedModelTypeFromTaskType(task.TaskType),
		RequiredModelFileHashes: executionHashes,
		QuantizationConfig:      quantizationConfig,
		MinVRAM:                 node.GPUVram,
	})
}
func StartLoadedModelFlush(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(loadedModelFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			flushLoadedModelCache(context.Background(), db)
			return
		case <-ticker.C:
			flushLoadedModelCache(ctx, db)
		}
	}
}

func flushLoadedModelCache(ctx context.Context, db *gorm.DB) {
	pending := loadedModelCache.take()
	if len(pending) == 0 {
		return
	}

	loadedModels := make([]models.LoadedModel, 0, len(pending))
	for key, pendingModel := range pending {
		loadedModels = append(loadedModels, models.LoadedModel{
			RequiredModelFilesHash:  key.RequiredModelFilesHash.Bytes(),
			ModelCacheHash:          key.ModelCacheHash.Bytes(),
			ExecutionDType:          key.ExecutionDType,
			RequiredModelFileHashes: models.EncodeHashList(pendingModel.RequiredModelFileHashes),
			QuantizationConfig:      pendingModel.QuantizationConfig,
			ModelID:                 pendingModel.ModelID,
			Variant:                 pendingModel.Variant,
			ModelType:               pendingModel.ModelType,
			MinVRAM:                 pendingModel.MinVRAM,
		})
	}
	if err := models.UpsertLoadedModelMinVRAMs(ctx, db, loadedModels); err != nil {
		log.Errorf("FlushLoadedModels: update loaded models error: %v", err)
		loadedModelCache.merge(pending)
	}
}

func (cache *loadedModelMinVRAMCache) record(key loadedModelKey, model pendingLoadedModel) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if current, ok := cache.pending[key]; !ok || model.MinVRAM < current.MinVRAM {
		cache.pending[key] = model
	}
}
func (cache *loadedModelMinVRAMCache) take() map[loadedModelKey]pendingLoadedModel {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	result := cache.pending
	cache.pending = make(map[loadedModelKey]pendingLoadedModel)
	return result
}

func (cache *loadedModelMinVRAMCache) merge(pending map[loadedModelKey]pendingLoadedModel) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for key, pendingModel := range pending {
		if current, ok := cache.pending[key]; !ok || pendingModel.MinVRAM < current.MinVRAM {
			cache.pending[key] = pendingModel
		}
	}
}
