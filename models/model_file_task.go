package models

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TaskModelRequirements is the model-file requirement derived from one task's TaskArgs.
type TaskModelRequirements struct {
	// Entries lists every independently downloadable model source of the task.
	Entries []ModelFileEntry
	// RequiredModelFileHashes holds the exact hashes of entries with a revision.
	RequiredModelFileHashes []Hash32
	// RequiredModelFileSourceHashes holds the source hashes of entries without a revision.
	RequiredModelFileSourceHashes []Hash32
	// RequestedModelCacheHash identifies the requested in-memory model; revisions do not participate.
	RequestedModelCacheHash Hash32
	// RequestedModelCacheConfig is the canonical JSON the cache hash is computed from.
	RequestedModelCacheConfig []byte
	// QuantizationConfigHash identifies the complete canonical quantization_config.
	QuantizationConfigHash Hash32
}

type taskArgsModelSource struct {
	Name     string  `json:"name"`
	Revision *string `json:"revision"`
}

type taskArgsComponent struct {
	Format       string         `json:"format"`
	Repo         string         `json:"repo"`
	Revision     *string        `json:"revision"`
	FormatKwargs map[string]any `json:"format_kwargs"`
}

type taskArgsLora struct {
	Model          string `json:"model"`
	WeightFileName string `json:"weight_file_name"`
	Weight         any    `json:"weight"`
}

type taskArgsModelFiles struct {
	Version            string                       `json:"version"`
	DType              string                       `json:"dtype"`
	QuantizationConfig any                          `json:"quantization_config"`
	Model              string                       `json:"model"`
	Revision           *string                      `json:"revision"`
	BaseModel          *taskArgsModelSource         `json:"base_model"`
	Components         map[string]taskArgsComponent `json:"components"`
	Lora               *taskArgsLora                `json:"lora"`
}

// QuantizationConfigHash hashes the canonical JSON of a complete quantization_config.
// A nil config hashes the canonical JSON null.
func QuantizationConfigHash(config any) (Hash32, error) {
	canonical, err := CanonicalJSON(config)
	if err != nil {
		return Hash32{}, err
	}
	return sha256.Sum256(canonical), nil
}

// DeriveTaskModelRequirements derives the required model files, the requested
// model cache hash, and the quantization config hash from TaskArgs.
func DeriveTaskModelRequirements(taskArgs string, taskType TaskType) (*TaskModelRequirements, error) {
	var args taskArgsModelFiles
	decoder := json.NewDecoder(strings.NewReader(taskArgs))
	decoder.UseNumber()
	if err := decoder.Decode(&args); err != nil {
		return nil, fmt.Errorf("invalid task args: %w", err)
	}

	cacheKey := map[string]any{
		"dtype": normalizeTaskDType(args.DType),
	}
	var entries []ModelFileEntry
	switch taskType {
	case TaskTypeLLM:
		if args.Model == "" {
			return nil, errors.New("task args model is empty")
		}
		entry := ModelFileEntry{
			SourceType:   ModelSourceTypeHuggingFace,
			Repository:   args.Model,
			Revision:     optionalString(args.Revision),
			FormatKwargs: map[string]any{},
		}
		entries = append(entries, entry)
		cacheKey["task_kind"] = "llm"
		cacheKey["model"] = entry.canonicalObject(false)
	case TaskTypeSD:
		if args.BaseModel == nil || args.BaseModel.Name == "" {
			return nil, errors.New("task args base_model name is empty")
		}
		base := ModelFileEntry{
			SourceType:   ModelSourceTypeHuggingFace,
			Repository:   args.BaseModel.Name,
			Revision:     optionalString(args.BaseModel.Revision),
			Format:       ImageBaseModelFormat,
			FormatKwargs: map[string]any{},
		}
		entries = append(entries, base)
		usages := map[string]any{"base_model": base.canonicalObject(false)}
		components := make(map[string]any, len(args.Components))
		for name, component := range args.Components {
			if name == "" || component.Format == "" || component.Repo == "" {
				return nil, fmt.Errorf("task args component %q requires name, format and repo", name)
			}
			entry := ModelFileEntry{
				SourceType:   ModelSourceTypeHuggingFace,
				Repository:   component.Repo,
				Revision:     optionalString(component.Revision),
				Format:       component.Format,
				FormatKwargs: component.FormatKwargs,
			}
			entries = append(entries, entry)
			components[name] = entry.canonicalObject(false)
		}
		cacheKey["task_kind"] = "image"
		cacheKey["base_model"] = usages["base_model"]
		cacheKey["components"] = components
		cacheKey["lora"] = canonicalLora(args.Lora)
	default:
		return nil, fmt.Errorf("task type %d has no model file requirements", taskType)
	}

	quantizationHash, err := QuantizationConfigHash(args.QuantizationConfig)
	if err != nil {
		return nil, err
	}
	cacheKey["quantization_config"] = args.QuantizationConfig

	cacheKeyJSON, err := CanonicalJSON(cacheKey)
	if err != nil {
		return nil, err
	}

	requirements := &TaskModelRequirements{
		Entries:                   entries,
		RequestedModelCacheHash:   sha256.Sum256(cacheKeyJSON),
		RequestedModelCacheConfig: cacheKeyJSON,
		QuantizationConfigHash:    quantizationHash,
	}
	if err := requirements.deriveRequiredHashes(); err != nil {
		return nil, err
	}
	return requirements, nil
}

// deriveRequiredHashes rebuilds the exact and source hash sets from the entries.
func (r *TaskModelRequirements) deriveRequiredHashes() error {
	r.RequiredModelFileHashes = nil
	r.RequiredModelFileSourceHashes = nil
	for _, entry := range r.Entries {
		if entry.Revision != "" {
			hash, err := entry.ModelFileHash()
			if err != nil {
				return err
			}
			r.RequiredModelFileHashes = append(r.RequiredModelFileHashes, hash)
			continue
		}
		hash, err := entry.ModelFileSourceHash()
		if err != nil {
			return err
		}
		r.RequiredModelFileSourceHashes = append(r.RequiredModelFileSourceHashes, hash)
	}
	r.RequiredModelFileHashes = SortedUniqueHashes(r.RequiredModelFileHashes)
	r.RequiredModelFileSourceHashes = SortedUniqueHashes(r.RequiredModelFileSourceHashes)
	return nil
}

// ResolveRevisions replaces the revision of the entry at each given index and
// rebuilds the required hash sets. The requested model cache hash is unchanged
// because revisions do not participate in it.
func (r *TaskModelRequirements) ResolveRevisions(revisions map[int]string) error {
	for index, revision := range revisions {
		if index < 0 || index >= len(r.Entries) {
			return fmt.Errorf("entry index %d out of range", index)
		}
		r.Entries[index].Revision = revision
	}
	return r.deriveRequiredHashes()
}

// RequestedModelCacheConfig returns the canonical cache config of the task
// derived from its TaskArgs.
func (task *InferenceTask) RequestedModelCacheConfig() ([]byte, error) {
	requirements, err := DeriveTaskModelRequirements(task.TaskArgs, task.TaskType)
	if err != nil {
		return nil, err
	}
	return requirements.RequestedModelCacheConfig, nil
}

// ModelFiles returns the catalog rows of the entries that carry a revision.
func (r *TaskModelRequirements) ModelFiles() ([]ModelFile, error) {
	files := make([]ModelFile, 0, len(r.Entries))
	for _, entry := range r.Entries {
		if entry.Revision == "" {
			continue
		}
		file, err := NewModelFile(entry)
		if err != nil {
			return nil, err
		}
		files = append(files, *file)
	}
	return files, nil
}

// SetModelRequirements stores the derived requirement hashes on the task.
func (task *InferenceTask) SetModelRequirements(r *TaskModelRequirements) {
	task.RequiredModelFileHashes = EncodeHashList(r.RequiredModelFileHashes)
	task.RequiredModelFileSourceHashes = EncodeHashList(r.RequiredModelFileSourceHashes)
	task.RequestedModelCacheHash = r.RequestedModelCacheHash.Bytes()
	task.QuantizationConfigHash = r.QuantizationConfigHash.Bytes()
}

// RequiredModelFileHashList returns the exact model file hashes required by the task.
func (task *InferenceTask) RequiredModelFileHashList() ([]Hash32, error) {
	return DecodeHashList(task.RequiredModelFileHashes)
}

// RequiredModelFileSourceHashList returns the source hashes of the revision-less
// model files required by the task.
func (task *InferenceTask) RequiredModelFileSourceHashList() ([]Hash32, error) {
	return DecodeHashList(task.RequiredModelFileSourceHashes)
}

// ExecutionModelFileHashList returns the exact model file hashes frozen at task start.
func (task *InferenceTask) ExecutionModelFileHashList() ([]Hash32, error) {
	return DecodeHashList(task.ExecutionModelFileHashes)
}

// RequestedModelCacheHash32 returns the requested model cache hash as a Hash32.
func (task *InferenceTask) RequestedModelCacheHash32() (Hash32, error) {
	return Hash32FromBytes(task.RequestedModelCacheHash)
}

// QuantizationConfigHash32 returns the quantization config hash as a Hash32.
func (task *InferenceTask) QuantizationConfigHash32() (Hash32, error) {
	return Hash32FromBytes(task.QuantizationConfigHash)
}

// DistributionKey identifies the required model file set as one hash covering
// both the exact and the revision-less members.
func DistributionKey(exact, source []Hash32) Hash32 {
	members := make([]Hash32, 0, len(exact)+len(source))
	members = append(members, exact...)
	members = append(members, source...)
	return RequiredModelFilesHash(members)
}

func normalizeTaskDType(dtype string) string {
	if dtype == "" {
		return AutoExecutionDType
	}
	return strings.ToLower(dtype)
}

func optionalString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func canonicalLora(lora *taskArgsLora) any {
	if lora == nil {
		return nil
	}
	weight := lora.Weight
	if weight == nil {
		weight = json.Number("100")
	}
	return map[string]any{
		"model":            lora.Model,
		"weight_file_name": lora.WeightFileName,
		"weight":           weight,
	}
}

// HasRequiredModelFiles reports whether the requirements contain any entry.
func (r *TaskModelRequirements) HasRequiredModelFiles() bool {
	return len(r.Entries) > 0
}

// TaskQuantizationConfigJSON returns the canonical JSON of the complete
// quantization_config of the TaskArgs; a missing config is canonical null.
func TaskQuantizationConfigJSON(taskArgs string) ([]byte, error) {
	var args taskArgsModelFiles
	decoder := json.NewDecoder(strings.NewReader(taskArgs))
	decoder.UseNumber()
	if err := decoder.Decode(&args); err != nil {
		return nil, fmt.Errorf("invalid task args: %w", err)
	}
	return CanonicalJSON(args.QuantizationConfig)
}
