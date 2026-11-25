package service

import (
	"bytes"
	"context"
	"crynux_relay/models"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
}

// LoadedModelPublicFields are the TaskArgs-shaped fields exposed by the public
// loaded-models view.
type LoadedModelPublicFields struct {
	Components map[string]any
	Lora       any
	Loras      []any
}

// LoadedModelView is a loaded model together with the number of nodes that
// hold its model files on disk and in memory.
type LoadedModelView struct {
	Model             models.LoadedModel
	Public            LoadedModelPublicFields
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
	Identity string
}

// loadedModelGroup collects the exact-file rows that share one public identity.
type loadedModelGroup struct {
	view              LoadedModelView
	members           [][]models.Hash32
	memberCacheHashes map[models.Hash32]struct{}
	sortKey           []byte
}

// loadedModelCacheConfigComplete reports whether the row carries the canonical
// cache config that its model cache hash is computed from.
func loadedModelCacheConfigComplete(loadedModel *models.LoadedModel, cacheHash models.Hash32) bool {
	config := []byte(loadedModel.ModelCacheConfig)
	return len(config) > 0 && json.Valid(config) && sha256.Sum256(config) == [32]byte(cacheHash)
}

type loadedModelCacheConfig struct {
	TaskKind           string         `json:"task_kind"`
	DType              string         `json:"dtype"`
	InputMode          string         `json:"input_mode"`
	QuantizationConfig any            `json:"quantization_config"`
	Model              map[string]any `json:"model"`
	BaseModel          map[string]any `json:"base_model"`
	Components         map[string]any `json:"components"`
	Lora               any            `json:"lora"`
	Loras              []any          `json:"loras"`
}

func parseLoadedModelCacheConfig(raw []byte) (*loadedModelCacheConfig, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var config loadedModelCacheConfig
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	dtype, err := models.ParseConcreteTaskDType(config.DType)
	if err != nil {
		return nil, err
	}
	config.DType = dtype
	return &config, nil
}

func taskArgsComponent(raw any) (map[string]any, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("component is not an object")
	}
	format, _ := object["format"].(string)
	repo, _ := object["repository"].(string)
	if repo == "" {
		repo, _ = object["repo"].(string)
	}
	formatKwargs, _ := object["format_kwargs"].(map[string]any)
	if formatKwargs == nil {
		formatKwargs = map[string]any{}
	}
	return map[string]any{
		"format":        format,
		"repo":          models.NormalizeModelName(repo),
		"format_kwargs": formatKwargs,
	}, nil
}

func publicFieldsFromCacheConfig(modelType models.LoadedModelType, config *loadedModelCacheConfig) (LoadedModelPublicFields, error) {
	fields := LoadedModelPublicFields{}
	switch modelType {
	case models.LoadedModelTypeLLM:
		return fields, nil
	case models.LoadedModelTypeSD:
		components := make(map[string]any, len(config.Components))
		for name, raw := range config.Components {
			component, err := taskArgsComponent(raw)
			if err != nil {
				return LoadedModelPublicFields{}, err
			}
			components[name] = component
		}
		fields.Components = components
		if config.Lora == nil {
			fields.Lora = nil
		} else {
			fields.Lora = config.Lora
		}
		return fields, nil
	case models.LoadedModelTypeVideo:
		components := make(map[string]any, len(config.Components))
		for name, raw := range config.Components {
			component, err := taskArgsComponent(raw)
			if err != nil {
				return LoadedModelPublicFields{}, err
			}
			components[name] = component
		}
		fields.Components = components
		if config.Loras == nil {
			fields.Loras = []any{}
		} else {
			fields.Loras = config.Loras
		}
		return fields, nil
	default:
		return LoadedModelPublicFields{}, fmt.Errorf("unsupported model type %q", modelType)
	}
}

func publicLoadedModelIdentity(model *models.LoadedModel, fields LoadedModelPublicFields) (string, []byte, error) {
	identity := map[string]any{
		"model_id":            model.ModelID,
		"model_type":          model.ModelType,
		"dtype":               model.DType,
		"quantization_config": json.RawMessage(model.QuantizationConfig),
	}
	switch model.ModelType {
	case models.LoadedModelTypeSD:
		identity["components"] = fields.Components
		identity["lora"] = fields.Lora
	case models.LoadedModelTypeVideo:
		identity["components"] = fields.Components
		identity["loras"] = fields.Loras
	}
	canonical, err := models.CanonicalJSON(identity)
	if err != nil {
		return "", nil, err
	}
	return string(canonical), canonical, nil
}

// groupLoadedModels aggregates the exact-file rows by the public model identity.
// Video rows that differ only by input_mode are merged. Rows without a complete
// concrete-dtype cache config are skipped.
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
		config, err := parseLoadedModelCacheConfig([]byte(loadedModel.ModelCacheConfig))
		if err != nil {
			log.Warnf("LoadedModelViews: skip loaded model %d of %s: %v", loadedModel.ID, loadedModel.ModelID, err)
			continue
		}
		if loadedModel.DType != config.DType {
			log.Warnf("LoadedModelViews: skip loaded model %d of %s: dtype mismatch", loadedModel.ID, loadedModel.ModelID)
			continue
		}
		public, err := publicFieldsFromCacheConfig(loadedModel.ModelType, config)
		if err != nil {
			log.Warnf("LoadedModelViews: skip loaded model %d of %s: %v", loadedModel.ID, loadedModel.ModelID, err)
			continue
		}
		members, err := loadedModel.MemberHashes()
		if err != nil {
			return nil, err
		}
		identity, sortKey, err := publicLoadedModelIdentity(&loadedModel, public)
		if err != nil {
			return nil, err
		}

		key := loadedModelGroupKey{Identity: identity}
		group, ok := groups[key]
		if !ok {
			groups[key] = &loadedModelGroup{
				view: LoadedModelView{
					Model:  loadedModel,
					Public: public,
				},
				members:           [][]models.Hash32{members},
				memberCacheHashes: map[models.Hash32]struct{}{cacheHash: {}},
				sortKey:           sortKey,
			}
			continue
		}
		representative := &group.view.Model
		if representative.ModelID != loadedModel.ModelID || representative.ModelType != loadedModel.ModelType ||
			representative.DType != loadedModel.DType ||
			!bytes.Equal(representative.QuantizationConfig, loadedModel.QuantizationConfig) {
			log.Errorf("LoadedModelViews: skip loaded model %d of %s: conflicts with model %s of the same public identity",
				loadedModel.ID, loadedModel.ModelID, representative.ModelID)
			continue
		}
		representative.MinVRAM = min(representative.MinVRAM, loadedModel.MinVRAM)
		group.members = append(group.members, members)
		group.memberCacheHashes[cacheHash] = struct{}{}
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
		if a.ModelType != b.ModelType {
			return a.ModelType < b.ModelType
		}
		if a.DType != b.DType {
			return a.DType < b.DType
		}
		return bytes.Compare(sorted[i].sortKey, sorted[j].sortKey) < 0
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
		for _, hashes := range onDisk {
			if coversAnyMember(hashes, group.members) {
				view.OnDiskNodeCount++
			}
		}
		for address, files := range inMemory {
			if _, ok := group.memberCacheHashes[memoryCacheHashes[address]]; !ok {
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
	DType                   string
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
	if _, err := models.ParseConcreteTaskDType(task.DType); err != nil {
		return
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
	}
	loadedModelCache.record(key, pendingLoadedModel{
		ModelID:                 task.ModelName,
		ModelType:               models.LoadedModelTypeFromTaskType(task.TaskType),
		DType:                   task.DType,
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
			DType:                   pendingModel.DType,
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
