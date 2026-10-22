package service

import (
	"bytes"
	"context"
	"crynux_relay/models"
	"crypto/sha256"
	"encoding/json"
	"sort"
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

// loadedModelGroupKey is the identity of a public loaded model view.
type loadedModelGroupKey struct {
	ModelCacheHash models.Hash32
	ExecutionDType string
}

// loadedModelGroup collects the exact-file rows that share one cache identity.
type loadedModelGroup struct {
	view    LoadedModelView
	members [][]models.Hash32
}

// loadedModelCacheConfigComplete reports whether the row carries the canonical
// cache config that its model cache hash is computed from.
func loadedModelCacheConfigComplete(loadedModel *models.LoadedModel, cacheHash models.Hash32) bool {
	config := []byte(loadedModel.ModelCacheConfig)
	return len(config) > 0 && json.Valid(config) && sha256.Sum256(config) == [32]byte(cacheHash)
}

// groupLoadedModels aggregates the exact-file rows by model cache hash and
// execution dtype. Rows without a complete cache config are skipped.
func groupLoadedModels(loadedModels []models.LoadedModel) ([]*loadedModelGroup, error) {
	groups := make(map[loadedModelGroupKey]*loadedModelGroup)
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
		cacheHash, err := models.Hash32FromBytes(loadedModel.ModelCacheHash)
		if err != nil {
			return nil, err
		}
		if !loadedModelCacheConfigComplete(&loadedModel, cacheHash) {
			log.Warnf("LoadedModelViews: skip loaded model %d of %s: model cache config is incomplete", loadedModel.ID, loadedModel.ModelID)
			continue
		}
		members, err := loadedModel.MemberHashes()
		if err != nil {
			return nil, err
		}

		key := loadedModelGroupKey{ModelCacheHash: cacheHash, ExecutionDType: loadedModel.ExecutionDType}
		group, ok := groups[key]
		if !ok {
			groups[key] = &loadedModelGroup{view: LoadedModelView{Model: loadedModel}, members: [][]models.Hash32{members}}
			continue
		}
		representative := &group.view.Model
		if representative.ModelID != loadedModel.ModelID || representative.ModelType != loadedModel.ModelType ||
			!bytes.Equal(representative.QuantizationConfig, loadedModel.QuantizationConfig) {
			log.Errorf("LoadedModelViews: skip loaded model %d of %s: conflicts with model %s of the same model cache hash %s",
				loadedModel.ID, loadedModel.ModelID, representative.ModelID, cacheHash.Hex())
			continue
		}
		representative.MinVRAM = min(representative.MinVRAM, loadedModel.MinVRAM)
		group.members = append(group.members, members)
	}

	sorted := make([]*loadedModelGroup, 0, len(groups))
	for _, group := range groups {
		sorted = append(sorted, group)
	}
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i].view.Model, sorted[j].view.Model
		if a.ModelID != b.ModelID {
			return a.ModelID < b.ModelID
		}
		if a.ExecutionDType != b.ExecutionDType {
			return a.ExecutionDType < b.ExecutionDType
		}
		return bytes.Compare(a.ModelCacheHash, b.ModelCacheHash) < 0
	})
	return sorted, nil
}

// coversAnyMember reports whether the held files contain every file of at least one member.
func coversAnyMember(held map[models.Hash32]struct{}, members [][]models.Hash32) bool {
	for _, member := range members {
		if modelFilesCovered(held, nil, member, nil) {
			return true
		}
	}
	return false
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
	inMemory := make(map[string]map[models.Hash32]struct{}, len(state.inMemory))
	memoryCacheHashes := make(map[string]models.Hash32, len(state.inMemory))
	for address, memory := range state.inMemory {
		cacheHash, err := models.Hash32FromBytes(memory.CurrentModelCacheHash)
		if err != nil {
			return nil, err
		}
		files, err := memory.FileHashes()
		if err != nil {
			return nil, err
		}
		inMemory[address] = hashSet(files)
		memoryCacheHashes[address] = cacheHash
	}

	groups, err := groupLoadedModels(loadedModels)
	if err != nil {
		return nil, err
	}
	views := make([]LoadedModelView, 0, len(groups))
	for _, group := range groups {
		view := group.view
		cacheHash, err := models.Hash32FromBytes(view.Model.ModelCacheHash)
		if err != nil {
			return nil, err
		}
		for _, hashes := range onDisk {
			if coversAnyMember(hashes, group.members) {
				view.OnDiskNodeCount++
			}
		}
		for address, files := range inMemory {
			if state.inMemory[address].ExecutionDType != view.Model.ExecutionDType || memoryCacheHashes[address] != cacheHash {
				continue
			}
			if coversAnyMember(files, group.members) {
				view.InMemoryNodeCount++
			}
		}
		views = append(views, view)
	}
	return views, nil
}

type pendingLoadedModel struct {
	ModelID                 string
	ModelType               models.LoadedModelType
	RequiredModelFileHashes []models.Hash32
	QuantizationConfig      []byte
	ModelCacheConfig        []byte
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
	cacheConfig, err := task.RequestedModelCacheConfig()
	if err != nil {
		log.Errorf("UpdateLoadedModels: derive model cache config of task %s error: %v", task.TaskIDCommitment, err)
		return
	}
	if sha256.Sum256(cacheConfig) != [32]byte(cacheHash) {
		log.Errorf("UpdateLoadedModels: model cache config of task %s does not match its model cache hash", task.TaskIDCommitment)
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
		ModelType:               models.LoadedModelTypeFromTaskType(task.TaskType),
		RequiredModelFileHashes: executionHashes,
		QuantizationConfig:      quantizationConfig,
		ModelCacheConfig:        cacheConfig,
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
			ModelCacheConfig:        pendingModel.ModelCacheConfig,
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
