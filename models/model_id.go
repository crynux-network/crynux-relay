package models

import (
	"encoding/json"
	"strings"
)

func NormalizeModelID(modelID string) string {
	return strings.ToLower(modelID)
}

func NormalizeModelIDs(modelIDs []string) []string {
	normalized := make([]string, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		normalized = append(normalized, NormalizeModelID(modelID))
	}
	return normalized
}

func IsBaseModelID(modelID string) bool {
	return strings.HasPrefix(NormalizeModelID(modelID), "base:")
}

func BaseModelIDs(modelIDs []string) []string {
	baseModelIDs := make([]string, 0, len(modelIDs))
	seen := make(map[string]struct{})
	for _, modelID := range modelIDs {
		normalized := NormalizeModelID(modelID)
		if !IsBaseModelID(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		baseModelIDs = append(baseModelIDs, normalized)
	}
	return baseModelIDs
}

// BaseModelHuggingFaceID extracts the huggingface model name from a base
// model dispatch ID formatted as "base:<name>" or "base:<name>+<variant>".
// It returns false for non-base model IDs (lora, controlnet) and for
// URL-based model names, which are not huggingface models.
func BaseModelHuggingFaceID(modelID string) (string, bool) {
	name, ok := strings.CutPrefix(modelID, "base:")
	if !ok {
		return "", false
	}
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return "", false
	}
	if variantSep := strings.IndexByte(name, '+'); variantSep >= 0 {
		name = name[:variantSep]
	}
	if name == "" {
		return "", false
	}
	return name, true
}

// NormalizeModelName lowercases a huggingface model name so that names
// differing only in letter case map to the same model. URL-based model names
// are kept unchanged because URL paths are case sensitive.
func NormalizeModelName(name string) string {
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
		return name
	}
	return strings.ToLower(name)
}

// NormalizeTaskArgsModelNames rewrites the model name fields inside the task
// args json string with NormalizeModelName, so that the model names the nodes
// use during inference match the normalized model ids used for model
// dispatching and downloading.
func NormalizeTaskArgsModelNames(taskArgs string, taskType TaskType) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(taskArgs))
	decoder.UseNumber()
	var argsMap map[string]interface{}
	if err := decoder.Decode(&argsMap); err != nil {
		return "", err
	}

	switch taskType {
	case TaskTypeSD:
		if baseModel, ok := argsMap["base_model"].(string); ok {
			argsMap["base_model"] = NormalizeModelName(baseModel)
		} else {
			normalizeObjectModelName(argsMap, "base_model", "name")
		}
		normalizeObjectModelName(argsMap, "lora", "model")
		normalizeObjectModelName(argsMap, "controlnet", "model")
		normalizeObjectModelName(argsMap, "refiner", "model")
		normalizeStringModelName(argsMap, "unet")
		normalizeStringModelName(argsMap, "vae")
		normalizeStringModelName(argsMap, "textual_inversion")
	case TaskTypeLLM:
		if model, ok := argsMap["model"].(string); ok {
			argsMap["model"] = NormalizeModelName(model)
		}
	case TaskTypeSDFTLora:
		normalizeObjectModelName(argsMap, "model", "name")
	}

	normalized, err := json.Marshal(argsMap)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func normalizeStringModelName(argsMap map[string]interface{}, key string) {
	if name, ok := argsMap[key].(string); ok {
		argsMap[key] = NormalizeModelName(name)
	}
}

func normalizeObjectModelName(argsMap map[string]interface{}, objectKey, nameKey string) {
	object, ok := argsMap[objectKey].(map[string]interface{})
	if !ok {
		return
	}
	if name, ok := object[nameKey].(string); ok {
		object[nameKey] = NormalizeModelName(name)
	}
}
