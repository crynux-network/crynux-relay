package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGPUFLOPSRequiresValidConfig(t *testing.T) {
	oldConfig := gpuFLOPSConfig
	t.Cleanup(func() {
		gpuFLOPSConfig = oldConfig
	})

	path := writeGPUFLOPSTestConfig(t, `{
  "default_gflops": 100,
  "gpus": [
    { "name": "rtx 4090", "vram": 24, "gflops": 84000 }
  ]
}`)

	if err := LoadGPUFLOPS(path); err != nil {
		t.Fatalf("failed to load gpu flops config: %v", err)
	}
	if gpuFLOPSConfig.DefaultGFLOPS != 100 {
		t.Fatalf("expected default GFLOPS 100, got %f", gpuFLOPSConfig.DefaultGFLOPS)
	}
}

func TestLoadGPUFLOPSRejectsInvalidConfig(t *testing.T) {
	oldConfig := gpuFLOPSConfig
	t.Cleanup(func() {
		gpuFLOPSConfig = oldConfig
	})

	path := writeGPUFLOPSTestConfig(t, `{
  "default_gflops": 0,
  "gpus": [
    { "name": "rtx 4090", "vram": 24, "gflops": 84000 }
  ]
}`)

	if err := LoadGPUFLOPS(path); err == nil {
		t.Fatal("expected invalid gpu flops config to fail")
	}
}

func TestCalculateTotalGFLOPSMatchesGPUNameSubstring(t *testing.T) {
	loadGPUFLOPSTestConfig(t, `{
  "default_gflops": 100,
  "gpus": [
    { "name": "rtx pro 6000", "vram": 96, "gflops": 128000 },
    { "name": "rtx 4090", "vram": 24, "gflops": 84000 },
    { "name": "rtx 4090", "vram": 48, "gflops": 84000 },
    { "name": "rtx 4090 d", "vram": 24, "gflops": 75000 },
    { "name": "rtx 4060", "vram": 8, "gflops": 15000 },
    { "name": "rtx 4060 laptop gpu", "gflops": 14000 },
    { "name": "rtx 4060 ti", "vram": 8, "gflops": 22000 }
  ]
}`)

	nodes := []NetworkNodeData{
		{CardModel: "NVIDIA RTX PRO 6000 Blackwell Workstation Edition", VRam: 96},
		{CardModel: "NVIDIA GeForce RTX 4090 D", VRam: 24},
		{CardModel: "NVIDIA GeForce RTX 4060 Ti", VRam: 8},
		{CardModel: "NVIDIA GeForce RTX 4060 Laptop GPU", VRam: 8},
	}

	got := CalculateTotalGFLOPS(nodes)
	want := 128000.0 + 75000.0 + 22000.0 + 14000.0
	if got != want {
		t.Fatalf("expected total GFLOPS %f, got %f", want, got)
	}
}

func TestCalculateTotalGFLOPSMultipliesGPUCountPrefix(t *testing.T) {
	loadGPUFLOPSTestConfig(t, `{
  "default_gflops": 100,
  "gpus": [
    { "name": "rtx 5090", "gflops": 107315.2 }
  ]
}`)

	nodes := []NetworkNodeData{
		{CardModel: "4x NVIDIA GeForce RTX 5090", VRam: 128},
		{CardModel: "NVIDIA GeForce RTX 5090", VRam: 32},
	}

	got := CalculateTotalGFLOPS(nodes)
	want := 4*107315.2 + 107315.2
	if got != want {
		t.Fatalf("expected total GFLOPS %f, got %f", want, got)
	}
}

func TestCalculateTotalGFLOPSEstimatesUnknownGPUByVRAMMedian(t *testing.T) {
	loadGPUFLOPSTestConfig(t, `{
  "default_gflops": 100,
  "gpus": [
    { "name": "gpu-a", "vram": 24, "gflops": 1000 },
    { "name": "gpu-b", "vram": 24, "gflops": 3000 },
    { "name": "gpu-c", "vram": 48, "gflops": 6000 }
  ]
}`)

	nodes := []NetworkNodeData{
		{CardModel: "GPU-A", VRam: 24},
		{CardModel: "GPU-B", VRam: 24},
		{CardModel: "unknown 32gb gpu", VRam: 32},
		{CardModel: "unknown 4gb gpu", VRam: 4},
	}

	got := CalculateTotalGFLOPS(nodes)
	want := 1000.0 + 3000.0 + 2000.0 + 100.0
	if got != want {
		t.Fatalf("expected total GFLOPS %f, got %f", want, got)
	}
}

func loadGPUFLOPSTestConfig(t *testing.T, content string) {
	t.Helper()

	oldConfig := gpuFLOPSConfig
	t.Cleanup(func() {
		gpuFLOPSConfig = oldConfig
	})

	if err := LoadGPUFLOPS(writeGPUFLOPSTestConfig(t, content)); err != nil {
		t.Fatalf("failed to load gpu flops config: %v", err)
	}
}

func writeGPUFLOPSTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "gpu_flops.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write gpu flops test config: %v", err)
	}
	return path
}
