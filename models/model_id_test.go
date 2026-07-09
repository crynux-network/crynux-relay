package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeModelID(t *testing.T) {
	modelID := "BaSe:Qwen/Qwen3.5-9B+FP16"
	got := NormalizeModelID(modelID)
	want := "base:qwen/qwen3.5-9b+fp16"
	if got != want {
		t.Fatalf("unexpected normalized model id, got %q, want %q", got, want)
	}
}

func TestNormalizeModelIDs(t *testing.T) {
	modelIDs := []string{
		"BaSe:Qwen/Qwen3.5-9B+FP16",
		"LoRa:Crynux-Network/MyLora+V1",
	}
	got := NormalizeModelIDs(modelIDs)
	want := []string{
		"base:qwen/qwen3.5-9b+fp16",
		"lora:crynux-network/mylora+v1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized model ids, got %v, want %v", got, want)
	}
}

func TestBaseModelHuggingFaceID(t *testing.T) {
	cases := []struct {
		modelID string
		want    string
		ok      bool
	}{
		{"base:qwen/qwen3-8b", "qwen/qwen3-8b", true},
		{"base:crynux-network/sdxl-turbo+fp16", "crynux-network/sdxl-turbo", true},
		{"lora:crynux-network/mylora", "", false},
		{"controlnet:lllyasviel/sd-controlnet-canny+fp16", "", false},
		{"base:https://example.com/models/mymodel.safetensors", "", false},
		{"base:", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := BaseModelHuggingFaceID(c.modelID)
		if got != c.want || ok != c.ok {
			t.Fatalf("BaseModelHuggingFaceID(%q) = (%q, %v), want (%q, %v)", c.modelID, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeModelName(t *testing.T) {
	if got := NormalizeModelName("Qwen/Qwen3.5-9B"); got != "qwen/qwen3.5-9b" {
		t.Fatalf("unexpected normalized model name: %q", got)
	}
	url := "https://example.com/Models/MyLora.safetensors"
	if got := NormalizeModelName(url); got != url {
		t.Fatalf("url model name should be unchanged, got %q", got)
	}
}

func assertTaskArgsJSON(t *testing.T, taskArgs string, taskType TaskType, wantArgs string) {
	t.Helper()
	got, err := NormalizeTaskArgsModelNames(taskArgs, taskType)
	if err != nil {
		t.Fatalf("NormalizeTaskArgsModelNames error: %v", err)
	}
	var gotMap, wantMap map[string]interface{}
	if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
		t.Fatalf("normalized task args is not valid json: %v", err)
	}
	if err := json.Unmarshal([]byte(wantArgs), &wantMap); err != nil {
		t.Fatalf("want task args is not valid json: %v", err)
	}
	if !reflect.DeepEqual(gotMap, wantMap) {
		t.Fatalf("unexpected normalized task args, got %s, want %s", got, wantArgs)
	}
}

func TestNormalizeTaskArgsModelNamesLLM(t *testing.T) {
	taskArgs := `{"model":"Qwen/Qwen2.5-7B","messages":[{"role":"user","content":"Hi"}],"generation_config":{"temperature":0.8},"seed":42}`
	wantArgs := `{"model":"qwen/qwen2.5-7b","messages":[{"role":"user","content":"Hi"}],"generation_config":{"temperature":0.8},"seed":42}`
	assertTaskArgsJSON(t, taskArgs, TaskTypeLLM, wantArgs)
}

func TestNormalizeTaskArgsModelNamesSD(t *testing.T) {
	taskArgs := `{"base_model":{"name":"Crynux-Network/SDXL-Turbo","variant":"fp16"},"prompt":"a cat","lora":{"model":"Crynux-Network/MyLora"},"controlnet":{"model":"Lllyasviel/Sd-Controlnet-Canny"},"refiner":{"model":"StabilityAI/Stable-Diffusion-XL-Refiner-1.0"},"unet":"Crynux-Network/MyUNet","vae":"MadeByOllin/SDXL-VAE-FP16-Fix","textual_inversion":"SD-Concepts-Library/Cat-Toy","task_config":{"num_images":1}}`
	wantArgs := `{"base_model":{"name":"crynux-network/sdxl-turbo","variant":"fp16"},"prompt":"a cat","lora":{"model":"crynux-network/mylora"},"controlnet":{"model":"lllyasviel/sd-controlnet-canny"},"refiner":{"model":"stabilityai/stable-diffusion-xl-refiner-1.0"},"unet":"crynux-network/myunet","vae":"madebyollin/sdxl-vae-fp16-fix","textual_inversion":"sd-concepts-library/cat-toy","task_config":{"num_images":1}}`
	assertTaskArgsJSON(t, taskArgs, TaskTypeSD, wantArgs)
}

func TestNormalizeTaskArgsModelNamesSDStringBaseModel(t *testing.T) {
	taskArgs := `{"base_model":"Crynux-Network/SDXL-Turbo","prompt":"a cat","task_config":{"num_images":1}}`
	wantArgs := `{"base_model":"crynux-network/sdxl-turbo","prompt":"a cat","task_config":{"num_images":1}}`
	assertTaskArgsJSON(t, taskArgs, TaskTypeSD, wantArgs)
}

func TestNormalizeTaskArgsModelNamesSDURLLora(t *testing.T) {
	taskArgs := `{"base_model":"Crynux-Network/SDXL-Turbo","prompt":"a cat","lora":{"model":"https://example.com/Models/MyLora.safetensors"},"task_config":{"num_images":1}}`
	wantArgs := `{"base_model":"crynux-network/sdxl-turbo","prompt":"a cat","lora":{"model":"https://example.com/Models/MyLora.safetensors"},"task_config":{"num_images":1}}`
	assertTaskArgsJSON(t, taskArgs, TaskTypeSD, wantArgs)
}

func TestNormalizeTaskArgsModelNamesSDFTLora(t *testing.T) {
	taskArgs := `{"model":{"name":"Crynux-Network/Stable-Diffusion-v1-5","variant":"fp16"},"dataset_name":"lambdalabs/naruto-blip-captions"}`
	wantArgs := `{"model":{"name":"crynux-network/stable-diffusion-v1-5","variant":"fp16"},"dataset_name":"lambdalabs/naruto-blip-captions"}`
	assertTaskArgsJSON(t, taskArgs, TaskTypeSDFTLora, wantArgs)
}
