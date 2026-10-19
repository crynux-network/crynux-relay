package service

import (
	"context"
	"crynux_relay/models"
	"crynux_relay/service/huggingface"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	runtimeConstraintSourceHubConfig      = "hub_config"
	runtimeConstraintSourceHubModelIndex  = "hub_model_index"
	runtimeConstraintSourceHubTransformer = "hub_transformer_config"
	defaultRuntimeConstraintTTL           = 5 * time.Minute
)

type runtimeConstraintMemoryEntry struct {
	requireLinuxCuda bool
	source           string
	expiresAt        time.Time
}

var (
	runtimeConstraintMemMu    sync.Mutex
	runtimeConstraintMemCache = map[string]runtimeConstraintMemoryEntry{}
)

func runtimeConstraintCacheKey(kind, repository, revision string) string {
	return kind + "|" + strings.ToLower(repository) + "|" + revision
}

func getRuntimeConstraintMemory(key string) (runtimeConstraintMemoryEntry, bool) {
	runtimeConstraintMemMu.Lock()
	defer runtimeConstraintMemMu.Unlock()
	entry, ok := runtimeConstraintMemCache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return runtimeConstraintMemoryEntry{}, false
	}
	return entry, true
}

func putRuntimeConstraintMemory(key string, requireLinuxCuda bool, source string, ttl time.Duration) {
	runtimeConstraintMemMu.Lock()
	defer runtimeConstraintMemMu.Unlock()
	runtimeConstraintMemCache[key] = runtimeConstraintMemoryEntry{
		requireLinuxCuda: requireLinuxCuda,
		source:           source,
		expiresAt:        time.Now().Add(ttl),
	}
}

// ClearRuntimeConstraintMemoryForTest clears the process-local detection cache.
func ClearRuntimeConstraintMemoryForTest() {
	runtimeConstraintMemMu.Lock()
	defer runtimeConstraintMemMu.Unlock()
	runtimeConstraintMemCache = map[string]runtimeConstraintMemoryEntry{}
}

func isLinuxCudaOnlyQuantMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "awq", "gptq", "gptqmodel", "nunchaku", "nunchaku_lite":
		return true
	default:
		return false
	}
}

func isTaskArgsLinuxCudaOnlyQuantMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "awq", "gptq", "gptqmodel":
		return true
	default:
		return false
	}
}

func taskArgsQuantMethod(taskArgs string) (string, error) {
	var parsed struct {
		QuantizationConfig *struct {
			QuantMethod string `json:"quant_method"`
		} `json:"quantization_config"`
	}
	if err := json.Unmarshal([]byte(taskArgs), &parsed); err != nil {
		return "", err
	}
	if parsed.QuantizationConfig == nil {
		return "", nil
	}
	return parsed.QuantizationConfig.QuantMethod, nil
}

func quantMethodFromConfigJSON(raw map[string]any) string {
	cfg, ok := raw["quantization_config"].(map[string]any)
	if !ok || cfg == nil {
		return ""
	}
	method, _ := cfg["quant_method"].(string)
	return method
}

func hubCacheTTL() time.Duration {
	return defaultRuntimeConstraintTTL
}

// ResolveRequireLinuxCuda detects whether the task may run only on docker or
// Linux GPU-name platforms. TaskArgs methods take precedence; otherwise Hub
// embedded quantization metadata is consulted through process and DB caches.
func ResolveRequireLinuxCuda(
	ctx context.Context,
	db *gorm.DB,
	taskType models.TaskType,
	taskArgs string,
	requirements *models.TaskModelRequirements,
) (bool, error) {
	method, err := taskArgsQuantMethod(taskArgs)
	if err != nil {
		return false, err
	}
	if isTaskArgsLinuxCudaOnlyQuantMethod(method) {
		return true, nil
	}

	switch taskType {
	case models.TaskTypeLLM:
		if len(requirements.Entries) == 0 {
			return false, nil
		}
		entry := requirements.Entries[0]
		return resolveHubLinuxCudaConstraint(ctx, db, "llm", entry.Repository, entry.Revision, detectLLMLinuxCuda)
	case models.TaskTypeSD, models.TaskTypeSDFTLora:
		if len(requirements.Entries) == 0 {
			return false, nil
		}
		entry := requirements.Entries[0]
		return resolveHubLinuxCudaConstraint(ctx, db, "sd", entry.Repository, entry.Revision, detectSDNunchakuLinuxCuda)
	default:
		return false, nil
	}
}

type hubLinuxCudaDetector func(ctx context.Context, client *huggingface.Client, repository, revision string) (bool, string, error)

func resolveHubLinuxCudaConstraint(
	ctx context.Context,
	db *gorm.DB,
	kind, repository, revision string,
	detect hubLinuxCudaDetector,
) (bool, error) {
	if strings.TrimSpace(revision) == "" {
		revision = "main"
	}
	memKey := runtimeConstraintCacheKey(kind, repository, revision)
	if entry, ok := getRuntimeConstraintMemory(memKey); ok {
		return entry.requireLinuxCuda, nil
	}

	row, ok, err := models.GetModelRuntimeConstraint(ctx, db, repository, revision)
	if err != nil {
		return false, err
	}
	if ok {
		putRuntimeConstraintMemory(memKey, row.RequireLinuxCuda, row.Source, hubCacheTTL())
		return row.RequireLinuxCuda, nil
	}

	client := getHuggingFaceClient()
	requireLinuxCuda, source, err := detect(ctx, client, repository, revision)
	if err != nil {
		return false, hubValidationError(err)
	}

	putRuntimeConstraintMemory(memKey, requireLinuxCuda, source, hubCacheTTL())
	if err := models.UpsertModelRuntimeConstraint(ctx, db, &models.ModelRuntimeConstraint{
		Repository:       repository,
		Revision:         revision,
		RequireLinuxCuda: requireLinuxCuda,
		Source:           source,
		DetectedAt:       time.Now().UTC(),
	}); err != nil {
		return false, err
	}
	return requireLinuxCuda, nil
}

func detectLLMLinuxCuda(ctx context.Context, client *huggingface.Client, repository, revision string) (bool, string, error) {
	var cfg map[string]any
	if err := client.FetchRepoFileJSON(ctx, repository, revision, "config.json", &cfg); err != nil {
		return false, "", err
	}
	method := quantMethodFromConfigJSON(cfg)
	if isLinuxCudaOnlyQuantMethod(method) {
		return true, runtimeConstraintSourceHubConfig, nil
	}
	return false, runtimeConstraintSourceHubConfig, nil
}

func detectSDNunchakuLinuxCuda(ctx context.Context, client *huggingface.Client, repository, revision string) (bool, string, error) {
	var index map[string]any
	if err := client.FetchRepoFileJSON(ctx, repository, revision, "model_index.json", &index); err != nil {
		return false, "", err
	}
	if method := quantMethodFromConfigJSON(index); isLinuxCudaOnlyQuantMethod(method) {
		return true, runtimeConstraintSourceHubModelIndex, nil
	}

	componentNames := []string{"transformer", "unet"}
	for _, name := range componentNames {
		if _, ok := index[name]; !ok {
			continue
		}
		var componentCfg map[string]any
		path := name + "/config.json"
		if err := client.FetchRepoFileJSON(ctx, repository, revision, path, &componentCfg); err != nil {
			if err == huggingface.ErrModelNotFound || err == huggingface.ErrRevisionNotFound {
				continue
			}
			return false, "", err
		}
		if method := quantMethodFromConfigJSON(componentCfg); isLinuxCudaOnlyQuantMethod(method) {
			return true, runtimeConstraintSourceHubTransformer, nil
		}
	}
	return false, runtimeConstraintSourceHubModelIndex, nil
}
