package service

import (
	"bytes"
	"crynux_relay/config"
	"crynux_relay/models"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/big"
	"strings"

	_ "golang.org/x/image/webp"
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

type llmWorkload struct {
	textInputBytes uint64
	imageCount     uint64
	imagePixels    uint64
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

func computeLLMWorkload(taskArgs string) (llmWorkload, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(taskArgs))
	decoder.UseNumber()
	var args map[string]interface{}
	if err := decoder.Decode(&args); err != nil {
		return llmWorkload{}, fmt.Errorf("parse llm task args: %w", err)
	}
	messages, workload, err := stripLLMImagePayloads(args["messages"])
	if err != nil {
		return llmWorkload{}, err
	}
	canonical := map[string]interface{}{
		"messages":      messages,
		"tools":         args["tools"],
		"template_args": args["template_args"],
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return llmWorkload{}, fmt.Errorf("encode canonical llm input: %w", err)
	}
	workload.textInputBytes = uint64(len(encoded))
	return workload, nil
}

func stripLLMImagePayloads(value interface{}) (interface{}, llmWorkload, error) {
	switch value := value.(type) {
	case []interface{}:
		result := make([]interface{}, len(value))
		var total llmWorkload
		for i := range value {
			cleaned, workload, err := stripLLMImagePayloads(value[i])
			if err != nil {
				return nil, llmWorkload{}, err
			}
			result[i] = cleaned
			if total.imageCount > math.MaxUint64-workload.imageCount ||
				total.imagePixels > math.MaxUint64-workload.imagePixels {
				return nil, llmWorkload{}, errors.New("llm image workload overflows uint64")
			}
			total.imageCount += workload.imageCount
			total.imagePixels += workload.imagePixels
		}
		return result, total, nil
	case map[string]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			result[key] = item
		}
		if imageType, _ := value["type"].(string); imageType == "image" {
			rawBase64, ok := value["base64"].(string)
			if !ok {
				return nil, llmWorkload{}, errors.New("llm image block base64 is not a string")
			}
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawBase64))
			if err != nil {
				return nil, llmWorkload{}, fmt.Errorf("decode llm image base64: %w", err)
			}
			imageConfig, _, err := image.DecodeConfig(bytes.NewReader(decoded))
			if err != nil {
				return nil, llmWorkload{}, fmt.Errorf("read llm image dimensions: %w", err)
			}
			if imageConfig.Width <= 0 || imageConfig.Height <= 0 {
				return nil, llmWorkload{}, errors.New("llm image dimensions must be positive")
			}
			width, height := uint64(imageConfig.Width), uint64(imageConfig.Height)
			if width > math.MaxUint64/height {
				return nil, llmWorkload{}, errors.New("llm image pixels overflow uint64")
			}
			delete(result, "base64")
			return result, llmWorkload{imageCount: 1, imagePixels: width * height}, nil
		}
		var total llmWorkload
		for key, item := range result {
			cleaned, workload, err := stripLLMImagePayloads(item)
			if err != nil {
				return nil, llmWorkload{}, err
			}
			result[key] = cleaned
			if total.imageCount > math.MaxUint64-workload.imageCount ||
				total.imagePixels > math.MaxUint64-workload.imagePixels {
				return nil, llmWorkload{}, errors.New("llm image workload overflows uint64")
			}
			total.imageCount += workload.imageCount
			total.imagePixels += workload.imagePixels
		}
		return result, total, nil
	default:
		return value, llmWorkload{}, nil
	}
}

func computeLLMInputBytes(taskArgs string) (uint64, error) {
	workload, err := computeLLMWorkload(taskArgs)
	return workload.textInputBytes, err
}

func computeEstimatedNodeSeconds(task *models.InferenceTask, parameters executionParameters, modelSwitch ...bool) (float64, error) {
	switch task.TaskType {
	case models.TaskTypeSD:
		if task.SDUnits == nil {
			return 0, errors.New("sd_units is not set")
		}
		return config.GetConfig().TaskPricing.OverheadSeconds + float64(*task.SDUnits)*parameters.sdRate, nil
	case models.TaskTypeLLM:
		if task.LLMTextInputBytes == nil {
			return 0, errors.New("llm_text_input_bytes is not set")
		}
		if task.LLMMaxNewTokens == nil {
			return 0, errors.New("llm_max_new_tokens is not set")
		}
		if task.LLMImageCount == nil {
			return 0, errors.New("llm_image_count is not set")
		}
		if task.LLMImagePixels == nil {
			return 0, errors.New("llm_image_pixels is not set")
		}
		switchWork := 0.0
		if len(modelSwitch) > 0 && modelSwitch[0] {
			switchWork = 1
		}
		return parameters.llm.constantSeconds +
			float64(*task.LLMTextInputBytes)*parameters.llm.secondsPerInputByte +
			float64(*task.LLMMaxNewTokens)*parameters.llm.secondsPerOutputToken +
			switchWork*parameters.llm.modelSwitchSeconds +
			float64(*task.LLMImageCount)*parameters.llm.secondsPerImage +
			(float64(*task.LLMImagePixels)/1_000_000)*parameters.llm.secondsPerMegapixel, nil
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
		task.LLMTextInputBytes = nil
		task.LLMImageCount = nil
		task.LLMImagePixels = nil
		task.LLMMaxNewTokens = nil
		task.PricingUnits = float64(units)
	case models.TaskTypeLLM:
		workload, err := computeLLMWorkload(task.TaskArgs)
		if err != nil {
			return err
		}
		maxNewTokens, err := computeLLMMaxNewTokens(task.TaskArgs)
		if err != nil {
			return err
		}
		task.LLMInputBytes = &workload.textInputBytes
		task.LLMTextInputBytes = &workload.textInputBytes
		task.LLMImageCount = &workload.imageCount
		task.LLMImagePixels = &workload.imagePixels
		task.LLMMaxNewTokens = &maxNewTokens
		task.SDUnits = nil
		task.PricingUnits = float64(maxNewTokens)
	case models.TaskTypeSDFTLora:
		task.SDUnits = nil
		task.LLMInputBytes = nil
		task.LLMTextInputBytes = nil
		task.LLMImageCount = nil
		task.LLMImagePixels = nil
		task.LLMMaxNewTokens = nil
		task.PricingUnits = 0
	default:
		return fmt.Errorf("unknown task type %d", task.TaskType)
	}
	var parameters executionParameters
	if task.TaskType == models.TaskTypeSD || task.TaskType == models.TaskTypeLLM {
		parameters = getTaskPricingParameters(task)
	}
	estimatedNodeSeconds, err := computeEstimatedNodeSeconds(task, parameters, false)
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
