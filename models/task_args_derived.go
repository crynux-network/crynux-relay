package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ParseTaskArgsVersion reads TaskArgs.version and requires major.minor.patch numeric form.
func ParseTaskArgsVersion(taskArgs string) (string, error) {
	var args struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
		return "", fmt.Errorf("invalid task args: %w", err)
	}
	if args.Version == "" {
		return "", errors.New("task args version is missing")
	}
	if err := ValidateTaskVersionFormat(args.Version); err != nil {
		return "", err
	}
	return args.Version, nil
}

// ValidateTaskVersionFormat accepts only major.minor.patch with numeric parts.
func ValidateTaskVersionFormat(version string) error {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid task version %q", version)
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return fmt.Errorf("invalid task version %q", version)
		}
	}
	return nil
}

// DeriveTaskSize returns the number of result files for a task.
// SD uses task_config.num_images_per_prompt (default 1). LLM and SDFT use 1.
func DeriveTaskSize(taskArgs string, taskType TaskType) (uint64, error) {
	switch taskType {
	case TaskTypeSD:
		var args struct {
			TaskConfig struct {
				NumImagesPerPrompt *uint64 `json:"num_images_per_prompt"`
			} `json:"task_config"`
		}
		if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
			return 0, fmt.Errorf("invalid task args: %w", err)
		}
		if args.TaskConfig.NumImagesPerPrompt == nil {
			return 1, nil
		}
		if *args.TaskConfig.NumImagesPerPrompt == 0 {
			return 0, errors.New("task_config.num_images_per_prompt must be greater than 0")
		}
		return *args.TaskConfig.NumImagesPerPrompt, nil
	case TaskTypeLLM, TaskTypeSDFTLora:
		return 1, nil
	default:
		return 0, fmt.Errorf("unsupported task type %d", taskType)
	}
}

// DeriveTaskModelIDs builds the legacy model-id list stored on the task row from TaskArgs.
func DeriveTaskModelIDs(taskArgs string, taskType TaskType) ([]string, error) {
	switch taskType {
	case TaskTypeLLM:
		var args struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
			return nil, fmt.Errorf("invalid task args: %w", err)
		}
		if args.Model == "" {
			return nil, errors.New("task args model is empty")
		}
		return NormalizeModelIDs([]string{"base:" + args.Model}), nil
	case TaskTypeSD:
		var args struct {
			BaseModel struct {
				Name string `json:"name"`
			} `json:"base_model"`
			Lora *struct {
				Model string `json:"model"`
			} `json:"lora"`
			Components map[string]struct {
				Repo string `json:"repo"`
			} `json:"components"`
		}
		if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
			return nil, fmt.Errorf("invalid task args: %w", err)
		}
		if args.BaseModel.Name == "" {
			return nil, errors.New("task args base_model name is empty")
		}
		ids := []string{"base:" + args.BaseModel.Name}
		if args.Lora != nil {
			if args.Lora.Model == "" {
				return nil, errors.New("task args lora.model is empty")
			}
			ids = append(ids, "lora:"+args.Lora.Model)
		}
		names := make([]string, 0, len(args.Components))
		for name := range args.Components {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			repo := args.Components[name].Repo
			if repo == "" {
				return nil, fmt.Errorf("task args components.%s.repo is empty", name)
			}
			ids = append(ids, "component:"+name+":"+repo)
		}
		return NormalizeModelIDs(ids), nil
	case TaskTypeSDFTLora:
		var args struct {
			Model struct {
				Name    string `json:"name"`
				Variant string `json:"variant"`
			} `json:"model"`
		}
		if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
			return nil, fmt.Errorf("invalid task args: %w", err)
		}
		if args.Model.Name == "" {
			return nil, errors.New("task args model.name is empty")
		}
		id := "base:" + args.Model.Name
		if args.Model.Variant != "" {
			id += "+" + args.Model.Variant
		}
		return NormalizeModelIDs([]string{id}), nil
	default:
		return nil, fmt.Errorf("unsupported task type %d", taskType)
	}
}
