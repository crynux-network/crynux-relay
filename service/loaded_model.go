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
var loadedModelNodeCountCache = &baseDispatchNodeCountCache{}

type loadedModelKey struct {
	ModelID string
	Variant string
}

type baseDispatchNodeCountCache struct {
	mu        sync.Mutex
	counts    map[string]models.HFModelNodeCount
	expiresAt time.Time
}

// GetLoadedModelNodeCounts returns per base-dispatch-ID node counts aggregated
// from the node_models table, cached in memory so that public API traffic does
// not translate into database load.
func GetLoadedModelNodeCounts(ctx context.Context, db *gorm.DB) (map[string]models.HFModelNodeCount, error) {
	return loadedModelNodeCountCache.get(ctx, db, time.Now())
}

func (cache *baseDispatchNodeCountCache) get(ctx context.Context, db *gorm.DB, now time.Time) (map[string]models.HFModelNodeCount, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.counts != nil && now.Before(cache.expiresAt) {
		return cache.counts, nil
	}
	counts, err := models.CountNodesByBaseDispatchID(ctx, db)
	if err != nil {
		return nil, err
	}
	cache.counts = counts
	cache.expiresAt = now.Add(loadedModelNodeCountTTL)
	return counts, nil
}

type pendingLoadedModel struct {
	ModelType models.LoadedModelType
	MinVRAM   uint64
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

func updateLoadedModels(task *models.InferenceTask, node *models.Node) {
	modelType := models.LoadedModelTypeFromTaskType(task.TaskType)
	seen := make(map[loadedModelKey]struct{}, len(task.ModelIDs))
	for _, modelID := range task.ModelIDs {
		hfModelID, ok := models.BaseModelHuggingFaceID(modelID)
		if !ok {
			continue
		}
		variant, ok := models.BaseModelVariant(modelID)
		if !ok {
			continue
		}
		key := loadedModelKey{ModelID: hfModelID, Variant: variant}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		loadedModelCache.record(key, modelType, node.GPUVram)
	}
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
			ModelID:   key.ModelID,
			Variant:   key.Variant,
			ModelType: pendingModel.ModelType,
			MinVRAM:   pendingModel.MinVRAM,
		})
	}
	if err := models.UpsertLoadedModelMinVRAMs(ctx, db, loadedModels); err != nil {
		log.Errorf("FlushLoadedModels: update loaded models error: %v", err)
		loadedModelCache.merge(pending)
	}
}

func (cache *loadedModelMinVRAMCache) record(key loadedModelKey, modelType models.LoadedModelType, minVRAM uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if current, ok := cache.pending[key]; !ok || minVRAM < current.MinVRAM {
		cache.pending[key] = pendingLoadedModel{ModelType: modelType, MinVRAM: minVRAM}
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
