package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateBaseModelLLMExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/org/llm" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siblings": []map[string]string{{"rfilename": "config.json"}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second, time.Minute)
	if err := client.ValidateBaseModel(context.Background(), ModelKindLLM, "org/llm"); err != nil {
		t.Fatalf("expected llm model to pass: %v", err)
	}
}

func TestValidateBaseModelSDDefaultWeights(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siblings": []map[string]string{
				{"rfilename": "model_index.json"},
				{"rfilename": "unet/diffusion_pytorch_model.fp16.safetensors"},
				{"rfilename": "unet/diffusion_pytorch_model.safetensors"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second, time.Minute)
	if err := client.ValidateBaseModel(context.Background(), ModelKindSD, "org/sd"); err != nil {
		t.Fatalf("expected default weights to pass: %v", err)
	}
}

func TestValidateBaseModelSDMissingDefaultWeights(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siblings": []map[string]string{
				{"rfilename": "model_index.json"},
				{"rfilename": "unet/diffusion_pytorch_model.fp16.safetensors"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second, time.Minute)
	err := client.ValidateBaseModel(context.Background(), ModelKindSD, "org/sd")
	if err != ErrDefaultWeightsNotFound {
		t.Fatalf("expected ErrDefaultWeightsNotFound, got %v", err)
	}
}

func TestValidateBaseModelNotFoundAndUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/models/missing/model" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second, time.Minute)
	if err := client.ValidateBaseModel(context.Background(), ModelKindLLM, "missing/model"); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
	if err := client.ValidateBaseModel(context.Background(), ModelKindLLM, "org/down"); err != ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestFetchRepoFileJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/org/model/resolve/main/config.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"quantization_config":{"quant_method":"gptq"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second, time.Minute)
	var cfg map[string]any
	if err := client.FetchRepoFileJSON(context.Background(), "org/model", "", "config.json", &cfg); err != nil {
		t.Fatalf("FetchRepoFileJSON: %v", err)
	}
	qc, _ := cfg["quantization_config"].(map[string]any)
	if qc["quant_method"] != "gptq" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
