package service

import (
	"context"
	"crynux_relay/models"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const loadedModelFlushInterval = 5 * time.Second

var loadedModelCache = newLoadedModelMinVRAMCache()

type loadedModelMinVRAMCache struct {
	mu      sync.Mutex
	pending map[string]uint64
}

func newLoadedModelMinVRAMCache() *loadedModelMinVRAMCache {
	return &loadedModelMinVRAMCache{
		pending: make(map[string]uint64),
	}
}

func updateLoadedModels(task *models.InferenceTask, node *models.Node) {
	seenModelIDs := make(map[string]struct{}, len(task.ModelIDs))
	for _, modelID := range task.ModelIDs {
		if modelID == "" {
			continue
		}
		if _, ok := seenModelIDs[modelID]; ok {
			continue
		}
		seenModelIDs[modelID] = struct{}{}
		loadedModelCache.record(modelID, node.GPUVram)
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

func ListLoadedModels(ctx context.Context, db *gorm.DB) ([]models.LoadedModel, error) {
	loadedModels, err := models.ListLoadedModels(ctx, db)
	if err != nil {
		return nil, err
	}

	minVRAMByModelID := make(map[string]uint64, len(loadedModels))
	for _, loadedModel := range loadedModels {
		minVRAMByModelID[loadedModel.ModelID] = loadedModel.MinVRAM
	}
	for modelID, minVRAM := range loadedModelCache.snapshot() {
		if currentMin, ok := minVRAMByModelID[modelID]; !ok || minVRAM < currentMin {
			minVRAMByModelID[modelID] = minVRAM
		}
	}

	modelIDs := make([]string, 0, len(minVRAMByModelID))
	for modelID := range minVRAMByModelID {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	result := make([]models.LoadedModel, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		result = append(result, models.LoadedModel{
			ModelID: modelID,
			MinVRAM: minVRAMByModelID[modelID],
		})
	}
	return result, nil
}

func flushLoadedModelCache(ctx context.Context, db *gorm.DB) {
	pending := loadedModelCache.take()
	if len(pending) == 0 {
		return
	}

	loadedModels := make([]models.LoadedModel, 0, len(pending))
	for modelID, minVRAM := range pending {
		loadedModels = append(loadedModels, models.LoadedModel{
			ModelID: modelID,
			MinVRAM: minVRAM,
		})
	}
	if err := models.UpsertLoadedModelMinVRAMs(ctx, db, loadedModels); err != nil {
		log.Errorf("FlushLoadedModels: update loaded models error: %v", err)
		loadedModelCache.merge(pending)
	}
}

func (cache *loadedModelMinVRAMCache) record(modelID string, minVRAM uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if currentMin, ok := cache.pending[modelID]; !ok || minVRAM < currentMin {
		cache.pending[modelID] = minVRAM
	}
}

func (cache *loadedModelMinVRAMCache) snapshot() map[string]uint64 {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	result := make(map[string]uint64, len(cache.pending))
	for modelID, minVRAM := range cache.pending {
		result[modelID] = minVRAM
	}
	return result
}

func (cache *loadedModelMinVRAMCache) take() map[string]uint64 {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	result := cache.pending
	cache.pending = make(map[string]uint64)
	return result
}

func (cache *loadedModelMinVRAMCache) merge(pending map[string]uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for modelID, minVRAM := range pending {
		if currentMin, ok := cache.pending[modelID]; !ok || minVRAM < currentMin {
			cache.pending[modelID] = minVRAM
		}
	}
}
