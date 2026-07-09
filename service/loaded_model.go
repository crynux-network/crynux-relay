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

var loadedModelCache = newLoadedModelMinVRAMCache()

type pendingLoadedModel struct {
	ModelType models.LoadedModelType
	MinVRAM   uint64
}

type loadedModelMinVRAMCache struct {
	mu      sync.Mutex
	pending map[string]pendingLoadedModel
}

func newLoadedModelMinVRAMCache() *loadedModelMinVRAMCache {
	return &loadedModelMinVRAMCache{
		pending: make(map[string]pendingLoadedModel),
	}
}

func updateLoadedModels(task *models.InferenceTask, node *models.Node) {
	modelType := models.LoadedModelTypeFromTaskType(task.TaskType)
	seenModelIDs := make(map[string]struct{}, len(task.ModelIDs))
	for _, modelID := range task.ModelIDs {
		hfModelID, ok := models.BaseModelHuggingFaceID(modelID)
		if !ok {
			continue
		}
		if _, ok := seenModelIDs[hfModelID]; ok {
			continue
		}
		seenModelIDs[hfModelID] = struct{}{}
		loadedModelCache.record(hfModelID, modelType, node.GPUVram)
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
	for modelID, pendingModel := range pending {
		loadedModels = append(loadedModels, models.LoadedModel{
			ModelID:   modelID,
			ModelType: pendingModel.ModelType,
			MinVRAM:   pendingModel.MinVRAM,
		})
	}
	if err := models.UpsertLoadedModelMinVRAMs(ctx, db, loadedModels); err != nil {
		log.Errorf("FlushLoadedModels: update loaded models error: %v", err)
		loadedModelCache.merge(pending)
	}
}

func (cache *loadedModelMinVRAMCache) record(modelID string, modelType models.LoadedModelType, minVRAM uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if current, ok := cache.pending[modelID]; !ok || minVRAM < current.MinVRAM {
		cache.pending[modelID] = pendingLoadedModel{ModelType: modelType, MinVRAM: minVRAM}
	}
}

func (cache *loadedModelMinVRAMCache) take() map[string]pendingLoadedModel {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	result := cache.pending
	cache.pending = make(map[string]pendingLoadedModel)
	return result
}

func (cache *loadedModelMinVRAMCache) merge(pending map[string]pendingLoadedModel) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for modelID, pendingModel := range pending {
		if current, ok := cache.pending[modelID]; !ok || pendingModel.MinVRAM < current.MinVRAM {
			cache.pending[modelID] = pendingModel
		}
	}
}
