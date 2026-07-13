package service

import (
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	defaultSDNumImages   = 6
	defaultSDImageWidth  = 512
	defaultSDImageHeight = 512

	// Positive lower bound applied to estimated node seconds before the
	// priority division.
	minEstimatedNodeSeconds = 1.0
)

// taskPricingCalibration holds the calibrated unit durations updated from
// completed task execution times with an exponentially weighted moving
// average. Values are seeded from configured initials at startup.
type taskPricingCalibration struct {
	mu                 sync.RWMutex
	secondsPerSDUnit   float64
	secondsPerLLMToken float64
}

var globalTaskPricingCalibration = &taskPricingCalibration{}

func InitTaskPricing() {
	cfg := config.GetConfig().TaskPricing
	globalTaskPricingCalibration.mu.Lock()
	defer globalTaskPricingCalibration.mu.Unlock()
	globalTaskPricingCalibration.secondsPerSDUnit = cfg.InitialSecondsPerSDUnit
	globalTaskPricingCalibration.secondsPerLLMToken = cfg.InitialSecondsPerLLMToken
	metrics.TaskPricingSecondsPerSDUnit.Set(cfg.InitialSecondsPerSDUnit)
	metrics.TaskPricingSecondsPerLLMToken.Set(cfg.InitialSecondsPerLLMToken)
}

func getCalibratedUnitSeconds(taskType models.TaskType) float64 {
	globalTaskPricingCalibration.mu.RLock()
	defer globalTaskPricingCalibration.mu.RUnlock()
	if taskType == models.TaskTypeSD {
		return globalTaskPricingCalibration.secondsPerSDUnit
	}
	return globalTaskPricingCalibration.secondsPerLLMToken
}

type sdPricingTaskConfig struct {
	NumImages   *float64 `json:"num_images"`
	ImageWidth  *float64 `json:"image_width"`
	ImageHeight *float64 `json:"image_height"`
}

type sdPricingArgs struct {
	TaskConfig *sdPricingTaskConfig `json:"task_config"`
}

type llmPricingGenerationConfig struct {
	MaxNewTokens *float64 `json:"max_new_tokens"`
}

type llmPricingArgs struct {
	GenerationConfig *llmPricingGenerationConfig `json:"generation_config"`
}

// computeSDPricingUnits reads image count and size from validated SD task
// args, applying the schema defaults for absent fields:
// sd_units = num_images * image_width * image_height / (512 * 512).
func computeSDPricingUnits(taskArgs string) (float64, error) {
	var args sdPricingArgs
	if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
		return 0, fmt.Errorf("parse sd task args: %w", err)
	}
	numImages := float64(defaultSDNumImages)
	imageWidth := float64(defaultSDImageWidth)
	imageHeight := float64(defaultSDImageHeight)
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
	}
	return numImages * imageWidth * imageHeight / (defaultSDImageWidth * defaultSDImageHeight), nil
}

// computeLLMPricingUnits reads max_new_tokens from validated LLM task args,
// using the configured default when generation_config or max_new_tokens is
// absent: llm_units = max_new_tokens.
func computeLLMPricingUnits(taskArgs string) (float64, error) {
	var args llmPricingArgs
	if err := json.Unmarshal([]byte(taskArgs), &args); err != nil {
		return 0, fmt.Errorf("parse llm task args: %w", err)
	}
	if args.GenerationConfig != nil && args.GenerationConfig.MaxNewTokens != nil {
		return *args.GenerationConfig.MaxNewTokens, nil
	}
	return float64(config.GetConfig().TaskPricing.DefaultLLMMaxNewTokens), nil
}

func computeTaskPricingUnits(task *models.InferenceTask) (float64, error) {
	switch task.TaskType {
	case models.TaskTypeSD:
		return computeSDPricingUnits(task.TaskArgs)
	case models.TaskTypeLLM:
		return computeLLMPricingUnits(task.TaskArgs)
	case models.TaskTypeSDFTLora:
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown task type %d", task.TaskType)
	}
}

func computeEstimatedNodeSeconds(task *models.InferenceTask, pricingUnits float64) float64 {
	overheadSeconds := config.GetConfig().TaskPricing.OverheadSeconds
	var estimated float64
	switch task.TaskType {
	case models.TaskTypeSD, models.TaskTypeLLM:
		estimated = overheadSeconds + pricingUnits*getCalibratedUnitSeconds(task.TaskType)
	case models.TaskTypeSDFTLora:
		// The stored timeout is a self-enforcing upper bound for fine-tune
		// pricing: understating it makes the task hit the running timeout
		// before completion.
		estimated = float64(task.Timeout)
	}
	if estimated < minEstimatedNodeSeconds {
		estimated = minEstimatedNodeSeconds
	}
	return estimated
}

// computeTaskVRAMWeight maps the task hardware requirement to the scarcity
// multiplier: vram_weight = max(vram_demand, base_vram) / base_vram.
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

// ApplyTaskPricing computes and stores the immutable pricing fields on a task
// before it is persisted: pricing units, estimated node seconds, VRAM weight
// and the queue priority priority = task_fee / (estimated_node_seconds * vram_weight).
func ApplyTaskPricing(task *models.InferenceTask) error {
	pricingUnits, err := computeTaskPricingUnits(task)
	if err != nil {
		return err
	}
	estimatedNodeSeconds := computeEstimatedNodeSeconds(task, pricingUnits)
	vramWeight := computeTaskVRAMWeight(task)

	priorityFloat := new(big.Float).Quo(
		new(big.Float).SetInt(&task.TaskFee.Int),
		big.NewFloat(estimatedNodeSeconds*vramWeight),
	)
	priority, _ := priorityFloat.Int(nil)

	task.PricingUnits = pricingUnits
	task.EstimatedNodeSeconds = estimatedNodeSeconds
	task.VRAMWeight = vramWeight
	task.Priority = models.BigInt{Int: *priority}
	return nil
}

// UpdateTaskPricingCalibration updates the calibrated unit duration for the
// task's type from its measured execution duration:
// measured_unit_seconds = max(measured_execution_seconds - overhead_seconds, 0) / pricing_units
// new_value = alpha * measured_unit_seconds + (1 - alpha) * old_value
// Calibration affects only tasks created after the update.
func UpdateTaskPricingCalibration(task *models.InferenceTask) {
	if task.TaskType != models.TaskTypeSD && task.TaskType != models.TaskTypeLLM {
		return
	}
	if task.PricingUnits <= 0 {
		return
	}
	if !task.StartTime.Valid || !task.ScoreReadyTime.Valid {
		return
	}
	cfg := config.GetConfig().TaskPricing
	measuredExecutionSeconds := task.ScoreReadyTime.Time.Sub(task.StartTime.Time).Seconds()
	measuredUnitSeconds := measuredExecutionSeconds - cfg.OverheadSeconds
	if measuredUnitSeconds < 0 {
		measuredUnitSeconds = 0
	}
	measuredUnitSeconds /= task.PricingUnits

	alpha := cfg.CalibrationAlpha
	globalTaskPricingCalibration.mu.Lock()
	defer globalTaskPricingCalibration.mu.Unlock()
	if task.TaskType == models.TaskTypeSD {
		globalTaskPricingCalibration.secondsPerSDUnit = alpha*measuredUnitSeconds + (1-alpha)*globalTaskPricingCalibration.secondsPerSDUnit
		metrics.TaskPricingSecondsPerSDUnit.Set(globalTaskPricingCalibration.secondsPerSDUnit)
		log.Debugf("TaskPricing: calibrated seconds_per_sd_unit to %f from task %s", globalTaskPricingCalibration.secondsPerSDUnit, task.TaskIDCommitment)
	} else {
		globalTaskPricingCalibration.secondsPerLLMToken = alpha*measuredUnitSeconds + (1-alpha)*globalTaskPricingCalibration.secondsPerLLMToken
		metrics.TaskPricingSecondsPerLLMToken.Set(globalTaskPricingCalibration.secondsPerLLMToken)
		log.Debugf("TaskPricing: calibrated seconds_per_llm_token to %f from task %s", globalTaskPricingCalibration.secondsPerLLMToken, task.TaskIDCommitment)
	}
}
