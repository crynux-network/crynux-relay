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
	if err := client.ValidateBaseModel(context.Background(), ModelKindLLM, "org/llm", ""); err != nil {
		t.Fatalf("expected llm model to pass: %v", err)
	}
}

func TestValidateBaseModelSDVariantAndDefault(t *testing.T) {
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
	if err := client.ValidateBaseModel(context.Background(), ModelKindSD, "org/sd", "fp16"); err != nil {
		t.Fatalf("expected fp16 variant to pass: %v", err)
	}
	if err := client.ValidateBaseModel(context.Background(), ModelKindSD, "org/sd", ""); err != nil {
		t.Fatalf("expected default variant to pass: %v", err)
	}
}

func TestValidateBaseModelSDMissingVariant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siblings": []map[string]string{
				{"rfilename": "model_index.json"},
				{"rfilename": "unet/diffusion_pytorch_model.safetensors"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second, time.Minute)
	err := client.ValidateBaseModel(context.Background(), ModelKindSD, "org/sd", "fp16")
	if err != ErrVariantUnavailable {
		t.Fatalf("expected ErrVariantUnavailable, got %v", err)
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
	if err := client.ValidateBaseModel(context.Background(), ModelKindLLM, "missing/model", ""); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
	if err := client.ValidateBaseModel(context.Background(), ModelKindLLM, "org/down", ""); err != ErrServiceUnavailable {
		t.Fatalf("expected ErrServiceUnavailable, got %v", err)
	}
}
