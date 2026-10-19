package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"net/http"
	"testing"
)

func TestResolveRequireLinuxCudaFromTaskArgsAWQ(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	calls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	})

	args := `{"version":"3.7.0","model":"org/model","dtype":"float16","quantization_config":{"quant_method":"awq","components_to_quantize":[],"quant_kwargs":{}}}`
	requirements, err := models.DeriveTaskModelRequirements(args, models.TaskTypeLLM)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got, err := ResolveRequireLinuxCuda(context.Background(), db, models.TaskTypeLLM, args, requirements)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !got {
		t.Fatal("expected RequireLinuxCuda from TaskArgs awq")
	}
	if calls != 0 {
		t.Fatalf("expected no hub calls for TaskArgs awq, got %d", calls)
	}
}

func TestResolveRequireLinuxCudaFromTaskArgsBitsAndBytes(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/org/model/resolve/main/config.json" {
			writeJSON(w, map[string]any{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	args := `{"version":"3.7.0","model":"org/model","dtype":"bfloat16","quantization_config":{"quant_method":"bitsandbytes","components_to_quantize":["transformer"],"quant_kwargs":{"load_in_4bit":true}}}`
	requirements, err := models.DeriveTaskModelRequirements(args, models.TaskTypeLLM)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got, err := ResolveRequireLinuxCuda(context.Background(), db, models.TaskTypeLLM, args, requirements)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got {
		t.Fatal("expected bitsandbytes TaskArgs not to set RequireLinuxCuda")
	}
}

func TestResolveRequireLinuxCudaFromHubAWQ(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	hubCalls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		hubCalls++
		if r.URL.Path == "/org/awq/resolve/rev-1/config.json" {
			writeJSON(w, map[string]any{
				"quantization_config": map[string]any{"quant_method": "awq"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	args := llmTaskArgs("org/awq", "rev-1")
	requirements, err := models.DeriveTaskModelRequirements(args, models.TaskTypeLLM)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got, err := ResolveRequireLinuxCuda(context.Background(), db, models.TaskTypeLLM, args, requirements)
	if err != nil || !got {
		t.Fatalf("expected hub awq RequireLinuxCuda, got %v err=%v", got, err)
	}

	before := hubCalls
	got, err = ResolveRequireLinuxCuda(context.Background(), db, models.TaskTypeLLM, args, requirements)
	if err != nil || !got {
		t.Fatalf("expected cached hub awq RequireLinuxCuda, got %v err=%v", got, err)
	}
	if hubCalls != before {
		t.Fatalf("expected second resolve to use cache, hub calls %d -> %d", before, hubCalls)
	}
}

func TestResolveRequireLinuxCudaFromHubNunchaku(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/org/nunchaku/resolve/main/model_index.json":
			writeJSON(w, map[string]any{
				"_class_name": "ZImagePipeline",
				"transformer": []string{"diffusers", "ZImageTransformer2DModel"},
			})
		case "/org/nunchaku/resolve/main/transformer/config.json":
			writeJSON(w, map[string]any{
				"quantization_config": map[string]any{"quant_method": "nunchaku_lite"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	args := imageTaskArgs("org/nunchaku", "")
	requirements, err := models.DeriveTaskModelRequirements(args, models.TaskTypeSD)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got, err := ResolveRequireLinuxCuda(context.Background(), db, models.TaskTypeSD, args, requirements)
	if err != nil || !got {
		t.Fatalf("expected nunchaku RequireLinuxCuda, got %v err=%v", got, err)
	}
}

func TestResolveRequireLinuxCudaHubOutage(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

	args := llmTaskArgs("org/model", "rev-1")
	requirements, err := models.DeriveTaskModelRequirements(args, models.TaskTypeLLM)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	_, err = ResolveRequireLinuxCuda(context.Background(), db, models.TaskTypeLLM, args, requirements)
	if !errors.Is(err, ErrBaseModelCheckUnavailable) {
		t.Fatalf("expected temporary hub error, got %v", err)
	}
}
