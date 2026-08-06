package service

import (
	"bytes"
	"crynux_relay/config"
	"crynux_relay/models"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
)

const (
	defaultSDNumImages      = uint64(6)
	defaultSDImageWidth     = uint64(512)
	defaultSDImageHeight    = uint64(512)
	defaultSDSteps          = uint64(1)
	minEstimatedNodeSeconds = 1.0
)

type sdPricingTaskConfig struct {
	NumImages   *uint64 `json:"num_images"`
	ImageWidth  *uint64 `json:"image_width"`
	ImageHeight *uint64 `json:"image_height"`
	Steps       *uint64 `json:"steps"`
}

type sdPricingArgs struct {
	TaskConfig *sdPricingTaskConfig `json:"task_config"`
}

type llmPricingGenerationConfig struct {
	MaxNewTokens *uint64 `json:"max_new_tokens"`
}

type llmPricingArgs struct {
	GenerationConfig *llmPricingGenerationConfig `json:"generation_config"`
}

func computeSDPricingUnits(taskArgs string) (uint64, error) {
	var args sdPricingArgs
	if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
		return 0, fmt.Errorf("parse sd task args: %w", err)
	}
	numImages, imageWidth, imageHeight, steps := defaultSDNumImages, defaultSDImageWidth, defaultSDImageHeight, defaultSDSteps
	if args.TaskConfig != nil {
		if args.TaskConfig.NumImages != nil {
			numImages = *args.TaskConfig.NumImages
		}
		if args.TaskConfig.ImageWidth != nil {
			imageWidth = *args.TaskConfig.ImageWidth
		}
		if args.TaskConfig.ImageHeight != nil {
			imageHeight = *args.TaskConfig.ImageHeight
		}
		if args.TaskConfig.Steps != nil {
			steps = *args.TaskConfig.Steps
		}
	}
	if numImages == 0 || imageWidth == 0 || imageHeight == 0 || steps == 0 {
		return 0, errors.New("sd pixel-step factors must be positive")
	}
	if numImages > math.MaxUint64/imageWidth || numImages*imageWidth > math.MaxUint64/imageHeight || numImages*imageWidth*imageHeight > math.MaxUint64/steps {
		return 0, errors.New("sd pixel-step units overflow uint64")
	}
	return numImages * imageWidth * imageHeight * steps, nil
}

func computeLLMMaxNewTokens(taskArgs string) (uint64, error) {
	var args llmPricingArgs
	if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
		return 0, fmt.Errorf("parse llm task args: %w", err)
	}
	if args.GenerationConfig != nil && args.GenerationConfig.MaxNewTokens != nil {
		return *args.GenerationConfig.MaxNewTokens, nil
	}
	return config.GetConfig().TaskPricing.DefaultLLMMaxNewTokens, nil
}

func computeLLMInputBytes(taskArgs string) (uint64, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(taskArgs))
	decoder.UseNumber()
	var args map[string]interface{}
	if err := decoder.Decode(&args); err != nil {
		return 0, fmt.Errorf("parse llm task args: %w", err)
	}
	canonical := map[string]interface{}{
		"messages":      args["messages"],
		"tools":         args["tools"],
		"template_args": args["template_args"],
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return 0, fmt.Errorf("encode canonical llm input: %w", err)
	}
	return uint64(len(encoded)), nil
}

func computeEstimatedNodeSeconds(task *models.InferenceTask, parameters executionParameters) (float64, error) {
	switch task.TaskType {
	case models.TaskTypeSD:
		if task.SDUnits == nil {
			return 0, errors.New("sd_units is not set")
		}
		return config.GetConfig().TaskPricing.OverheadSeconds + float64(*task.SDUnits)*parameters.sdRate, nil
	case models.TaskTypeLLM:
		if task.LLMInputBytes == nil {
			return 0, errors.New("llm_input_bytes is not set")
		}
		if task.LLMMaxNewTokens == nil {
			return 0, errors.New("llm_max_new_tokens is not set")
		}
		return parameters.llm[0] + float64(*task.LLMInputBytes)*parameters.llm[1] + float64(*task.LLMMaxNewTokens)*parameters.llm[2], nil
	case models.TaskTypeSDFTLora:
		return float64(task.Timeout), nil
	default:
		return 0, fmt.Errorf("unknown task type %d", task.TaskType)
	}
}

func computeTaskVRAMWeight(task *models.InferenceTask) float64 {
	baseVRAM := config.GetConfig().TaskPricing.BaseVRAM
	var vramDemand uint64
	if len(task.RequiredGPU) > 0 {
		vramDemand = task.RequiredGPUVRAM
	} else {
		vramDemand = task.MinVRAM
	}
	if vramDemand < baseVRAM {
		vramDemand = baseVRAM
	}
	return float64(vramDemand) / float64(baseVRAM)
}

func ApplyTaskPricing(task *models.InferenceTask) error {
	switch task.TaskType {
	case models.TaskTypeSD:
		units, err := computeSDPricingUnits(task.TaskArgs)
		if err != nil {
			return err
		}
		task.SDUnits = &units
		task.LLMInputBytes = nil
		task.LLMMaxNewTokens = nil
		task.PricingUnits = float64(units)
	case models.TaskTypeLLM:
		inputBytes, err := computeLLMInputBytes(task.TaskArgs)
		if err != nil {
			return err
		}
		maxNewTokens, err := computeLLMMaxNewTokens(task.TaskArgs)
		if err != nil {
			return err
		}
		task.LLMInputBytes = &inputBytes
		task.LLMMaxNewTokens = &maxNewTokens
		task.SDUnits = nil
		task.PricingUnits = float64(maxNewTokens)
	case models.TaskTypeSDFTLora:
		task.SDUnits = nil
		task.LLMInputBytes = nil
		task.LLMMaxNewTokens = nil
		task.PricingUnits = 0
	default:
		return fmt.Errorf("unknown task type %d", task.TaskType)
	}
	var parameters executionParameters
	if task.TaskType == models.TaskTypeSD || task.TaskType == models.TaskTypeLLM {
		parameters = getTaskPricingParameters(task)
	}
	estimatedNodeSeconds, err := computeEstimatedNodeSeconds(task, parameters)
	if err != nil {
		return err
	}
	if estimatedNodeSeconds < minEstimatedNodeSeconds {
		estimatedNodeSeconds = minEstimatedNodeSeconds
	}
	vramWeight := computeTaskVRAMWeight(task)

	priorityFloat := new(big.Float).Quo(
		new(big.Float).SetInt(&task.TaskFee.Int),
		big.NewFloat(estimatedNodeSeconds*vramWeight),
	)
	priority, _ := priorityFloat.Int(nil)

	task.EstimatedNodeSeconds = estimatedNodeSeconds
	task.VRAMWeight = vramWeight
	task.Priority = models.BigInt{Int: *priority}
	return nil
}
