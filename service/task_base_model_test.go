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

func newTaskBaseModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelFile{}, &models.NodeModel{}, &models.ModelRuntimeConstraint{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ClearRuntimeConstraintMemoryForTest()
	t.Cleanup(ClearRuntimeConstraintMemoryForTest)
	return db
}

func useHuggingFaceServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	SetHuggingFaceClientForTest(huggingface.NewClient(server.URL, time.Second, time.Minute))
	t.Cleanup(func() { SetHuggingFaceClientForTest(nil) })
}

func seedRuntimeConstraint(t *testing.T, db *gorm.DB, repository, revision string, requireLinuxCuda bool) {
	t.Helper()
	if err := models.UpsertModelRuntimeConstraint(context.Background(), db, &models.ModelRuntimeConstraint{
		Repository:       repository,
		Revision:         revision,
		RequireLinuxCuda: requireLinuxCuda,
		Source:           "hub_config",
		DetectedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed runtime constraint: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, body any) {
	_ = json.NewEncoder(w).Encode(body)
}

func TestPrepareTaskModelRequirementsSkipsHubWhenNodeHasModel(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedNodeModelFiles(t, db, "0x1", testModelFile(t, "org/model", "rev-1"))
	seedRuntimeConstraint(t, db, "org/model", "rev-1", false)

	calls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", "rev-1"))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected hub not to be called, got %d calls", calls)
	}
	want := testModelFileHash(t, "org/model", "rev-1")
	if len(requirements.RequiredModelFileHashes) != 1 || requirements.RequiredModelFileHashes[0] != want {
		t.Fatalf("unexpected required hashes: %v", requirements.RequiredModelFileHashes)
	}
	if requirements.RequireLinuxCuda {
		t.Fatal("expected RequireLinuxCuda false from cached constraint")
	}
}

func TestPrepareTaskModelRequirementsResolvesRevisionToCommit(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/org/model/revision/main":
			writeJSON(w, map[string]string{"sha": "commit-sha"})
		case "/org/model/resolve/commit-sha/config.json":
			writeJSON(w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", "main"))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	want := testModelFileHash(t, "org/model", "commit-sha")
	if len(requirements.RequiredModelFileHashes) != 1 || requirements.RequiredModelFileHashes[0] != want {
		t.Fatalf("expected hash of the commit, got %v", requirements.RequiredModelFileHashes)
	}
	stored, err := models.GetModelFiles(context.Background(), db, []models.Hash32{want})
	if err != nil || len(stored) != 1 {
		t.Fatalf("expected the commit entry in the catalog, got %v, %v", stored, err)
	}
}

func TestPrepareTaskModelRequirementsRejectsMissingRevision(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Error-Code", "RevisionNotFound")
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", "missing"))
	if !errors.Is(err, ErrInvalidBaseModel) {
		t.Fatalf("expected ErrInvalidBaseModel, got %v", err)
	}
}

func TestPrepareTaskModelRequirementsRejectsMissingHubModel(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("missing/model", ""))
	if !errors.Is(err, ErrInvalidBaseModel) {
		t.Fatalf("expected ErrInvalidBaseModel, got %v", err)
	}
}

func TestPrepareTaskModelRequirementsTemporaryOnHubOutage(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

	_, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if !errors.Is(err, ErrBaseModelCheckUnavailable) {
		t.Fatalf("expected ErrBaseModelCheckUnavailable, got %v", err)
	}
}

func TestPrepareTaskModelRequirementsKeepsSourceIdentityWithoutRevision(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/org/sd":
			writeJSON(w, map[string]interface{}{
				"siblings": []map[string]string{
					{"rfilename": "model_index.json"},
					{"rfilename": "unet/diffusion_pytorch_model.safetensors"},
				},
			})
		case "/org/sd/resolve/main/model_index.json":
			writeJSON(w, map[string]any{"_class_name": "StableDiffusionPipeline"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeSD, imageTaskArgs("org/sd", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if len(requirements.RequiredModelFileHashes) != 0 || len(requirements.RequiredModelFileSourceHashes) != 1 {
		t.Fatalf("unexpected requirements: %+v", requirements)
	}
	wantSource, err := requirements.Entries[0].ModelFileSourceHash()
	if err != nil || requirements.RequiredModelFileSourceHashes[0] != wantSource {
		t.Fatalf("unexpected source hash: %v, %v", requirements.RequiredModelFileSourceHashes, err)
	}
}

func TestPrepareTaskModelRequirementsRejectsInvalidArgs(t *testing.T) {
	db := newTaskBaseModelTestDB(t)

	_, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, `{"version":"3.7.0"}`)
	if !errors.Is(err, ErrInvalidTaskModelArgs) {
		t.Fatalf("expected ErrInvalidTaskModelArgs, got %v", err)
	}
}

func TestPrepareTaskModelRequirementsAcceptsAnyFormattedVersion(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedNodeModelFiles(t, db, "0x1", testModelFile(t, "org/model", "rev-1"))
	seedRuntimeConstraint(t, db, "org/model", "rev-1", false)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	requirements, err := PrepareTaskModelRequirements(
		context.Background(), db, models.TaskTypeLLM,
		`{"version":"9.1.0","model":"org/model","revision":"rev-1"}`)
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if len(requirements.Entries) != 1 || requirements.Entries[0].Repository != "org/model" {
		t.Fatalf("unexpected requirements: %+v", requirements)
	}
}

func TestPrepareTaskModelRequirementsAcceptsDiffusersComponentWithoutModelIndex(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/org/sd":
			writeJSON(w, map[string]interface{}{
				"siblings": []map[string]string{
					{"rfilename": "model_index.json"},
					{"rfilename": "vae/config.json"},
					{"rfilename": "vae/diffusion_pytorch_model.safetensors"},
				},
			})
		case "/api/models/org/vae":
			writeJSON(w, map[string]interface{}{
				"siblings": []map[string]string{
					{"rfilename": "config.json"},
					{"rfilename": "diffusion_pytorch_model.safetensors"},
				},
			})
		case "/org/sd/resolve/main/model_index.json":
			writeJSON(w, map[string]any{"_class_name": "StableDiffusionPipeline", "transformer": []string{"diffusers", "Transformer2DModel"}})
		case "/org/sd/resolve/main/transformer/config.json":
			writeJSON(w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	taskArgs := `{"version":"3.7.0","base_model":{"name":"org/sd"},"components":{"vae":{"format":"diffusers","repo":"org/vae","format_kwargs":{}}},"dtype":"float16","prompt":"a cat"}`
	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeSD, taskArgs)
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if len(requirements.Entries) != 2 {
		t.Fatalf("expected base and component entries, got %+v", requirements.Entries)
	}
	if requirements.Entries[0].Repository != "org/sd" || requirements.Entries[0].Format != models.ImageBaseModelFormat {
		t.Fatalf("unexpected base entry: %+v", requirements.Entries[0])
	}
	if requirements.Entries[1].Repository != "org/vae" || requirements.Entries[1].Format != "diffusers" {
		t.Fatalf("unexpected component entry: %+v", requirements.Entries[1])
	}
}

func TestPrepareTaskModelRequirementsRejectsBaseWithoutModelIndex(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"siblings": []map[string]string{
				{"rfilename": "config.json"},
				{"rfilename": "diffusion_pytorch_model.safetensors"},
			},
		})
	})

	_, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeSD, imageTaskArgs("org/vae-only", ""))
	if !errors.Is(err, ErrInvalidBaseModel) {
		t.Fatalf("expected ErrInvalidBaseModel, got %v", err)
	}
}
