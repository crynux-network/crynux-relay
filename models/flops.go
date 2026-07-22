package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var gpuCountPrefixRegexp = regexp.MustCompile(`^(\d+)x\s+`)

type NetworkFLOPS struct {
	gorm.Model
	GFLOPS float64 `json:"gflops"`
}

type GPUFLOPSConfig struct {
	DefaultGFLOPS float64         `json:"default_gflops"`
	GPUs          []GPUFLOPSEntry `json:"gpus"`
	entries       []gpuFLOPSEntry
}

type GPUFLOPSEntry struct {
	Name   string  `json:"name"`
	VRAM   *int    `json:"vram,omitempty"`
	GFLOPS float64 `json:"gflops"`
}

type gpuFLOPSEntry struct {
	GPUFLOPSEntry
	normalizedName string
}

var gpuFLOPSConfig *GPUFLOPSConfig

func LoadGPUFLOPS(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read gpu flops file: %w", err)
	}

	conf := &GPUFLOPSConfig{}
	if err := json.Unmarshal(content, conf); err != nil {
		return fmt.Errorf("parse gpu flops file: %w", err)
	}
	if err := conf.validate(); err != nil {
		return err
	}

	gpuFLOPSConfig = conf
	return nil
}

func (conf *GPUFLOPSConfig) validate() error {
	if conf.DefaultGFLOPS <= 0 {
		return errors.New("gpu flops default_gflops must be greater than 0")
	}
	if len(conf.GPUs) == 0 {
		return errors.New("gpu flops gpus must not be empty")
	}

	entries := make([]gpuFLOPSEntry, 0, len(conf.GPUs))
	for i, gpu := range conf.GPUs {
		normalizedName := strings.ToLower(strings.TrimSpace(gpu.Name))
		if normalizedName == "" {
			return fmt.Errorf("gpu flops gpus[%d].name must not be empty", i)
		}
		if gpu.GFLOPS <= 0 {
			return fmt.Errorf("gpu flops gpus[%d].gflops must be greater than 0", i)
		}
		if gpu.VRAM != nil && *gpu.VRAM <= 0 {
			return fmt.Errorf("gpu flops gpus[%d].vram must be greater than 0", i)
		}
		entries = append(entries, gpuFLOPSEntry{
			GPUFLOPSEntry:  gpu,
			normalizedName: normalizedName,
		})
	}
	conf.entries = entries
	return nil
}

func CalculateTotalGFLOPS(nodes []NetworkNodeData) float64 {
	if gpuFLOPSConfig == nil {
		log.Error("GPU FLOPS config is not loaded")
		return 0
	}

	matched := make([]bool, len(nodes))
	matchedGFLOPS := make([]float64, len(nodes))
	vramSamples := make(map[int][]float64)

	for i, node := range nodes {
		gflops, ok := matchGPUGFLOPS(node.CardModel, node.VRam)
		if !ok {
			continue
		}
		matched[i] = true
		matchedGFLOPS[i] = gflops
		if node.VRam > 0 {
			vramSamples[node.VRam] = append(vramSamples[node.VRam], gflops)
		}
	}

	vramEstimates := medianGFLOPSByVRAM(vramSamples)

	var totalGFLOPS float64
	for i, node := range nodes {
		if matched[i] {
			totalGFLOPS += matchedGFLOPS[i]
			continue
		}

		gflops := estimateGFLOPSByVRAM(node.VRam, vramEstimates)
		totalGFLOPS += gflops
		log.WithFields(log.Fields{
			"card_model": node.CardModel,
			"vram":       node.VRam,
			"gflops":     gflops,
		}).Warn("GPU model not found in FLOPS config; using VRAM estimate")
	}

	return totalGFLOPS
}

func matchGPUGFLOPS(cardModel string, vram int) (float64, bool) {
	cardModel = strings.ToLower(strings.TrimSpace(cardModel))
	if cardModel == "" {
		return 0, false
	}

	gpuCount := 1
	if m := gpuCountPrefixRegexp.FindStringSubmatch(cardModel); m != nil {
		if count, err := strconv.Atoi(m[1]); err == nil && count > 0 {
			gpuCount = count
			cardModel = cardModel[len(m[0]):]
		}
	}

	bestIndex := -1
	bestHasVRAM := false
	for i, gpu := range gpuFLOPSConfig.entries {
		if !strings.Contains(cardModel, gpu.normalizedName) {
			continue
		}
		hasVRAM := gpu.VRAM != nil
		if hasVRAM && *gpu.VRAM != vram {
			continue
		}
		if bestIndex == -1 ||
			len(gpu.normalizedName) > len(gpuFLOPSConfig.entries[bestIndex].normalizedName) ||
			(len(gpu.normalizedName) == len(gpuFLOPSConfig.entries[bestIndex].normalizedName) && hasVRAM && !bestHasVRAM) {
			bestIndex = i
			bestHasVRAM = hasVRAM
		}
	}

	if bestIndex == -1 {
		return 0, false
	}
	return gpuFLOPSConfig.entries[bestIndex].GFLOPS * float64(gpuCount), true
}

func medianGFLOPSByVRAM(samples map[int][]float64) map[int]float64 {
	estimates := make(map[int]float64, len(samples))
	for vram, values := range samples {
		sort.Float64s(values)
		middle := len(values) / 2
		if len(values)%2 == 0 {
			estimates[vram] = (values[middle-1] + values[middle]) / 2
		} else {
			estimates[vram] = values[middle]
		}
	}
	return estimates
}

func estimateGFLOPSByVRAM(vram int, estimates map[int]float64) float64 {
	bestVRAM := 0
	for candidateVRAM := range estimates {
		if candidateVRAM <= vram && candidateVRAM > bestVRAM {
			bestVRAM = candidateVRAM
		}
	}
	if bestVRAM == 0 {
		return gpuFLOPSConfig.DefaultGFLOPS
	}
	return estimates[bestVRAM]
}
