package models

import "testing"

func TestProductionCardModelCoverage(t *testing.T) {
	oldConfig := gpuFLOPSConfig
	t.Cleanup(func() { gpuFLOPSConfig = oldConfig })

	if err := LoadGPUFLOPS("../config/gpu_flops.json"); err != nil {
		t.Fatalf("failed to load real gpu flops config: %v", err)
	}

	cases := []struct {
		cardModel string
		vram      int
		want      float64
	}{
		{"NVIDIA GeForce RTX 5090+docker", 32, 107315.2},
		{"Apple M4 Type+Darwin", 16, 4362.24},
		{"NVIDIA RTX PRO 6000 Blackwell Server Edition+docker", 96, 128000.0},
		{"NVIDIA GeForce RTX 3090+docker", 24, 36433.92},
		{"NVIDIA RTX PRO 6000 Blackwell Workstation Edition+docker", 96, 128000.0},
		{"NVIDIA GeForce RTX 4090+docker", 24, 84582.4},
		{"NVIDIA GeForce RTX 4090+docker", 48, 84582.4},
		{" Apple M4\n      Type+Darwin", 16, 4362.24},
		{"NVIDIA RTX PRO 6000 Blackwell Max-Q Workstation Edition+docker", 96, 128000.0},
		{"Tesla T4+docker", 15, 8336.4},
		{"NVIDIA GeForce RTX 4090+Windows", 24, 84582.4},
		{"4x NVIDIA GeForce RTX 5090+docker", 128, 4 * 107315.2},
		{"NVIDIA B300 SXM6 AC+docker", 269, 76800.0},
		{"Apple M4 Pro Type+Darwin", 64, 9437.184},
		{"NVIDIA GeForce RTX 5090+Linux", 32, 107315.2},
		{"NVIDIA GeForce RTX 3070 Laptop GPU+Windows", 8, 16988.0},
		{"NVIDIA H100 80GB HBM3+docker", 80, 68608.0},
		{"NVIDIA L40S+docker", 45, 93798.4},
		{"NVIDIA GeForce RTX 5070 Ti+docker", 16, 44953.6},
		{"NVIDIA RTX A4000+Windows", 16, 19170.0},
		{"NVIDIA GeForce RTX 4060 Ti+Windows", 16, 22630.4},
		{"NVIDIA GeForce RTX 5080+Windows", 16, 57651.2},
		{"NVIDIA GeForce RTX 5070+Windows", 12, 31641.6},
		{"NVIDIA GeForce RTX 4060 Laptop GPU+docker", 8, 14848.0},
		{"NVIDIA GeForce RTX 3080+Windows", 10, 31334.4},
		{"NVIDIA GeForce RTX 2050+Windows", 4, 5222.4},
		{"NVIDIA GeForce RTX 4070 Ti+Windows", 12, 41062.4},
		{"NVIDIA GeForce RTX 3050 Laptop GPU+Windows", 4, 7301.0},
		{"Apple M1 Max Type+Darwin", 64, 10649.6},
		{"NVIDIA GeForce RTX 4060+Windows", 8, 15462.4},
		{"NVIDIA GeForce RTX 5060 Ti+docker", 16, 24268.8},
		{"NVIDIA GeForce RTX 4080 Laptop GPU+Windows", 12, 34611.2},
		{"Apple M2 Pro Type+Darwin", 32, 6963.2},
		{"NVIDIA GeForce RTX 4070+Windows", 12, 29798.4},
		{"NVIDIA GeForce RTX 4070 SUPER+docker", 12, 36331.52},
		{"NVIDIA GeForce RTX 3060+Windows", 12, 13045.76},
		{" Apple M5 Pro\n      Type+Darwin", 24, 8488.96},
		{"NVIDIA GeForce RTX 4070 Ti SUPER+docker", 16, 45158.4},
		{"NVIDIA GeForce RTX 4090+docker", 23, 84582.4},
		{"NVIDIA GeForce RTX 5080+docker", 16, 57651.2},
		{"NVIDIA GeForce RTX 3090 Ti+docker", 24, 40960.0},
		{" Apple M2\n      Type+Darwin", 16, 3686.4},
		{"NVIDIA GeForce RTX 4070 Ti+docker", 12, 41062.4},
		{"NVIDIA B200+docker", 180, 81920.0},
		{"NVIDIA GeForce RTX 4090 D+docker", 24, 75264.0},
		{"NVIDIA RTX 6000 Ada Generation+docker", 48, 91060.0},
		{"NVIDIA RTX 4000 SFF Ada Generation+docker", 20, 19660.8},
		{"NVIDIA RTX 5000 Ada Generation Laptop GPU+Windows", 16, 42600.0},
		{"NVIDIA A100-SXM4-80GB+docker", 80, 19500.0},
		{"NVIDIA H100 NVL+docker", 94, 61440.0},
		{"Apple M3 Ultra Type+Darwin", 96, 28940.288},
	}

	for _, c := range cases {
		got, ok := matchGPUGFLOPS(c.cardModel, c.vram)
		if !ok {
			t.Errorf("no match for %q (vram %d)", c.cardModel, c.vram)
			continue
		}
		if got != c.want {
			t.Errorf("card %q (vram %d): got %f, want %f", c.cardModel, c.vram, got, c.want)
		}
	}
}
