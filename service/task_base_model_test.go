package service

import (
	"context"
	"crynux_relay/models"
	"crynux_relay/service/huggingface"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateTaskBaseModelSkipsHubWhenNodeHasModel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	nodeModel := models.NewNodeModel("0x1", "base:org/model+fp16", false)
	if err := db.Create(&nodeModel).Error; err != nil {
		t.Fatalf("create node model: %v", err)
	}

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	SetHuggingFaceClientForTest(huggingface.NewClient(server.URL, time.Second, time.Minute))
	defer SetHuggingFaceClientForTest(nil)

	err = ValidateTaskBaseModel(context.Background(), db, models.TaskTypeSD, []string{"base:org/model+fp16"})
	if err != nil {
		t.Fatalf("expected validation to pass: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected hub not to be called, got %d calls", calls)
	}
}

func TestValidateTaskBaseModelRejectsMissingHubModel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	SetHuggingFaceClientForTest(huggingface.NewClient(server.URL, time.Second, time.Minute))
	defer SetHuggingFaceClientForTest(nil)

	err = ValidateTaskBaseModel(context.Background(), db, models.TaskTypeLLM, []string{"base:missing/model"})
	if !errors.Is(err, ErrInvalidBaseModel) {
		t.Fatalf("expected ErrInvalidBaseModel, got %v", err)
	}
}

func TestValidateTaskBaseModelTemporaryOnHubOutage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	SetHuggingFaceClientForTest(huggingface.NewClient(server.URL, time.Second, time.Minute))
	defer SetHuggingFaceClientForTest(nil)

	err = ValidateTaskBaseModel(context.Background(), db, models.TaskTypeLLM, []string{"base:org/model"})
	if !errors.Is(err, ErrBaseModelCheckUnavailable) {
		t.Fatalf("expected ErrBaseModelCheckUnavailable, got %v", err)
	}
}

func TestValidateTaskBaseModelAcceptsValidSDFromHub(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.NodeModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siblings": []map[string]string{
				{"rfilename": "model_index.json"},
				{"rfilename": "unet/diffusion_pytorch_model.fp16.safetensors"},
			},
		})
	}))
	defer server.Close()
	SetHuggingFaceClientForTest(huggingface.NewClient(server.URL, time.Second, time.Minute))
	defer SetHuggingFaceClientForTest(nil)

	err = ValidateTaskBaseModel(context.Background(), db, models.TaskTypeSD, []string{"base:org/sd+fp16"})
	if err != nil {
		t.Fatalf("expected validation to pass: %v", err)
	}
}
