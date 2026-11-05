package service

import (
	"context"
	"crynux_relay/models"
	"crynux_relay/service/huggingface"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	testCommitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testCommitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testCommitC = "cccccccccccccccccccccccccccccccccccccccc"
	testCommitD = "dddddddddddddddddddddddddddddddddddddddd"
)

func newTaskBaseModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Node{},
		&models.ModelFile{},
		&models.NodeModel{},
		&models.NodeInMemoryModel{},
		&models.ModelRuntimeConstraint{},
	); err != nil {
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

func seedOnlineNode(t *testing.T, db *gorm.DB, address string, status models.NodeStatus) {
	t.Helper()
	node := models.Node{Address: address, Status: status}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
}

func seedOnlineNodeModel(t *testing.T, db *gorm.DB, address string, status models.NodeStatus, files ...models.ModelFile) {
	t.Helper()
	seedOnlineNode(t, db, address, status)
	seedNodeModelFiles(t, db, address, files...)
}

func seedNodeMemory(t *testing.T, db *gorm.DB, address string, files ...models.ModelFile) {
	t.Helper()
	hashes := make([]models.Hash32, 0, len(files))
	for _, file := range files {
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			t.Fatalf("invalid model file hash: %v", err)
		}
		hashes = append(hashes, hash)
	}
	memory := models.NewNodeInMemoryModel(address, models.Hash32{1}, hashes, []byte(`{}`), "auto")
	if err := models.SaveNodeInMemoryModel(context.Background(), db, &memory); err != nil {
		t.Fatalf("save in-memory model: %v", err)
	}
}

func assertExactCommit(t *testing.T, requirements *models.TaskModelRequirements, repository, commit string) {
	t.Helper()
	want := testModelFileHash(t, repository, commit)
	if len(requirements.RequiredModelFileHashes) != 1 || requirements.RequiredModelFileHashes[0] != want {
		t.Fatalf("unexpected required hashes: %v, want %x", requirements.RequiredModelFileHashes, want)
	}
	if len(requirements.RequiredModelFileSourceHashes) != 0 {
		t.Fatalf("expected empty source hashes, got %v", requirements.RequiredModelFileSourceHashes)
	}
	if len(requirements.Entries) != 1 || requirements.Entries[0].Revision != commit {
		t.Fatalf("unexpected entries: %+v", requirements.Entries)
	}
}

func TestPrepareTaskModelRequirementsSkipsHubWhenNodeHasCommit(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedOnlineNodeModel(t, db, "0x1", models.NodeStatusAvailable, testModelFile(t, "org/model", testCommitA))
	seedRuntimeConstraint(t, db, "org/model", testCommitA, false)

	calls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", testCommitA))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected hub not to be called, got %d calls", calls)
	}
	assertExactCommit(t, requirements, "org/model", testCommitA)
	if requirements.RequireLinuxCuda {
		t.Fatal("expected RequireLinuxCuda false from cached constraint")
	}
}

func TestPrepareTaskModelRequirementsNormalizesUppercaseCommit(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedOnlineNodeModel(t, db, "0x1", models.NodeStatusAvailable, testModelFile(t, "org/model", testCommitA))
	seedRuntimeConstraint(t, db, "org/model", testCommitA, false)

	calls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	upper := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", upper))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected hub not to be called when uppercase commit matches lowercase inventory, got %d", calls)
	}
	assertExactCommit(t, requirements, "org/model", testCommitA)
}

func TestPrepareTaskModelRequirementsResolvesRevisionToCommit(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/org/model/revision/main":
			writeJSON(w, map[string]string{"sha": testCommitA})
		case "/org/model/resolve/" + testCommitA + "/config.json":
			writeJSON(w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", "main"))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	assertExactCommit(t, requirements, "org/model", testCommitA)
	stored, err := models.GetModelFiles(context.Background(), db, []models.Hash32{testModelFileHash(t, "org/model", testCommitA)})
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

func TestPrepareTaskModelRequirementsFreezesEmptyRevisionFromHubMain(t *testing.T) {
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
		case "/api/models/org/sd/revision/main":
			writeJSON(w, map[string]string{"sha": testCommitA})
		case "/org/sd/resolve/" + testCommitA + "/model_index.json":
			writeJSON(w, map[string]any{"_class_name": "StableDiffusionPipeline"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeSD, imageTaskArgs("org/sd", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	want := testImageBaseModelFileHash(t, "org/sd", testCommitA)
	if len(requirements.RequiredModelFileHashes) != 1 || requirements.RequiredModelFileHashes[0] != want {
		t.Fatalf("unexpected required hashes: %v", requirements.RequiredModelFileHashes)
	}
	if len(requirements.RequiredModelFileSourceHashes) != 0 {
		t.Fatalf("expected empty source hashes, got %v", requirements.RequiredModelFileSourceHashes)
	}
	if requirements.Entries[0].Revision != testCommitA {
		t.Fatalf("expected frozen commit, got %q", requirements.Entries[0].Revision)
	}
}

func TestPrepareTaskModelRequirementsReusesSingleNetworkCommit(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedOnlineNodeModel(t, db, "0x1", models.NodeStatusAvailable, testModelFile(t, "org/model", testCommitA))
	seedRuntimeConstraint(t, db, "org/model", testCommitA, false)

	calls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected hub not to be called when reusing network commit, got %d", calls)
	}
	assertExactCommit(t, requirements, "org/model", testCommitA)
}

func TestPrepareTaskModelRequirementsPrefersLoadedRevision(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	fileA := testModelFile(t, "org/model", testCommitA)
	fileB := testModelFile(t, "org/model", testCommitB)
	seedOnlineNodeModel(t, db, "0xa1", models.NodeStatusAvailable, fileA)
	seedOnlineNodeModel(t, db, "0xa2", models.NodeStatusAvailable, fileA)
	seedOnlineNodeModel(t, db, "0xb1", models.NodeStatusAvailable, fileB)
	seedNodeMemory(t, db, "0xb1", fileB)
	seedRuntimeConstraint(t, db, "org/model", testCommitB, false)

	calls := 0
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected hub not to be called, got %d", calls)
	}
	assertExactCommit(t, requirements, "org/model", testCommitB)
}

func TestPrepareTaskModelRequirementsPrefersMoreDiskHolders(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	fileA := testModelFile(t, "org/model", testCommitA)
	fileB := testModelFile(t, "org/model", testCommitB)
	seedOnlineNodeModel(t, db, "0xa1", models.NodeStatusAvailable, fileA)
	seedOnlineNodeModel(t, db, "0xb1", models.NodeStatusAvailable, fileB)
	seedOnlineNodeModel(t, db, "0xb2", models.NodeStatusBusy, fileB)
	seedRuntimeConstraint(t, db, "org/model", testCommitB, false)

	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected hub call: %s", r.URL.Path)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	assertExactCommit(t, requirements, "org/model", testCommitB)
}

func TestPrepareTaskModelRequirementsStableHashTiebreak(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	fileA := testModelFile(t, "org/model", testCommitA)
	fileB := testModelFile(t, "org/model", testCommitB)
	seedOnlineNodeModel(t, db, "0xa", models.NodeStatusAvailable, fileA)
	seedOnlineNodeModel(t, db, "0xb", models.NodeStatusAvailable, fileB)
	hashA := testModelFileHash(t, "org/model", testCommitA)
	hashB := testModelFileHash(t, "org/model", testCommitB)
	wantCommit := testCommitA
	wantHash := hashA
	if bytesLess(hashB[:], hashA[:]) {
		wantCommit = testCommitB
		wantHash = hashB
	}
	seedRuntimeConstraint(t, db, "org/model", wantCommit, false)

	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected hub call: %s", r.URL.Path)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if requirements.RequiredModelFileHashes[0] != wantHash {
		t.Fatalf("expected stable hash tiebreak %x, got %x", wantHash, requirements.RequiredModelFileHashes[0])
	}
}

func bytesLess(a, b []byte) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func TestPrepareTaskModelRequirementsIgnoresOfflineNodes(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	fileA := testModelFile(t, "org/model", testCommitA)
	fileB := testModelFile(t, "org/model", testCommitB)
	seedOnlineNodeModel(t, db, "0xpaused", models.NodeStatusPaused, fileA)
	seedOnlineNodeModel(t, db, "0xquit", models.NodeStatusQuit, fileA)
	seedOnlineNodeModel(t, db, "0xonline", models.NodeStatusAvailable, fileB)
	seedRuntimeConstraint(t, db, "org/model", testCommitB, false)

	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected hub call: %s", r.URL.Path)
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	assertExactCommit(t, requirements, "org/model", testCommitB)
}

func TestPrepareTaskModelRequirementsIgnoresNonCommitInventory(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedOnlineNodeModel(t, db, "0xbranch", models.NodeStatusAvailable, testModelFile(t, "org/model", "main"))
	seedOnlineNodeModel(t, db, "0xtag", models.NodeStatusAvailable, testModelFile(t, "org/model", "v1.0"))
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/org/model/revision/main":
			writeJSON(w, map[string]string{"sha": testCommitC})
		case "/org/model/resolve/" + testCommitC + "/config.json":
			writeJSON(w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", ""))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	assertExactCommit(t, requirements, "org/model", testCommitC)
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
	seedOnlineNodeModel(t, db, "0x1", models.NodeStatusAvailable, testModelFile(t, "org/model", testCommitA))
	seedRuntimeConstraint(t, db, "org/model", testCommitA, false)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	requirements, err := PrepareTaskModelRequirements(
		context.Background(), db, models.TaskTypeLLM,
		`{"version":"9.1.0","model":"org/model","revision":"`+testCommitA+`"}`)
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
		case "/api/models/org/sd/revision/main":
			writeJSON(w, map[string]string{"sha": testCommitA})
		case "/api/models/org/vae/revision/main":
			writeJSON(w, map[string]string{"sha": testCommitB})
		case "/org/sd/resolve/" + testCommitA + "/model_index.json":
			writeJSON(w, map[string]any{"_class_name": "StableDiffusionPipeline", "transformer": []string{"diffusers", "Transformer2DModel"}})
		case "/org/sd/resolve/" + testCommitA + "/transformer/config.json":
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
	if requirements.Entries[0].Repository != "org/sd" || requirements.Entries[0].Format != models.ImageBaseModelFormat || requirements.Entries[0].Revision != testCommitA {
		t.Fatalf("unexpected base entry: %+v", requirements.Entries[0])
	}
	if requirements.Entries[1].Repository != "org/vae" || requirements.Entries[1].Format != "diffusers" || requirements.Entries[1].Revision != testCommitB {
		t.Fatalf("unexpected component entry: %+v", requirements.Entries[1])
	}
	if len(requirements.RequiredModelFileSourceHashes) != 0 {
		t.Fatalf("expected empty source hashes, got %v", requirements.RequiredModelFileSourceHashes)
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

func TestPrepareTaskModelRequirementsResolvesExplicitBranchEvenWhenHeld(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	seedOnlineNodeModel(t, db, "0x1", models.NodeStatusAvailable, testModelFile(t, "org/model", "main"))
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/org/model/revision/main":
			writeJSON(w, map[string]string{"sha": testCommitD})
		case "/org/model/resolve/" + testCommitD + "/config.json":
			writeJSON(w, map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeLLM, llmTaskArgs("org/model", "main"))
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	assertExactCommit(t, requirements, "org/model", testCommitD)
}

func TestPrepareTaskModelRequirementsGGUFComponentEmptyRevision(t *testing.T) {
	db := newTaskBaseModelTestDB(t)
	useHuggingFaceServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/models/org/sd":
			writeJSON(w, map[string]interface{}{
				"siblings": []map[string]string{
					{"rfilename": "model_index.json"},
					{"rfilename": "unet/diffusion_pytorch_model.safetensors"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/revision/main") && strings.Contains(r.URL.Path, "org/sd"):
			writeJSON(w, map[string]string{"sha": testCommitA})
		case strings.HasSuffix(r.URL.Path, "/revision/main") && strings.Contains(r.URL.Path, "org/gguf"):
			writeJSON(w, map[string]string{"sha": testCommitB})
		case strings.Contains(r.URL.Path, "/resolve/"+testCommitA+"/model_index.json"):
			writeJSON(w, map[string]any{"_class_name": "StableDiffusionPipeline"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	taskArgs := `{"version":"3.7.0","base_model":{"name":"org/sd"},"components":{"transformer":{"format":"gguf","repo":"org/gguf","format_kwargs":{"filename":"model.gguf"}}},"dtype":"float16","prompt":"a cat"}`
	requirements, err := PrepareTaskModelRequirements(context.Background(), db, models.TaskTypeSD, taskArgs)
	if err != nil {
		t.Fatalf("expected preparation to pass: %v", err)
	}
	if len(requirements.Entries) != 2 {
		t.Fatalf("expected base and gguf entries, got %+v", requirements.Entries)
	}
	if requirements.Entries[1].Revision != testCommitB || requirements.Entries[1].Format != "gguf" {
		t.Fatalf("unexpected gguf entry: %+v", requirements.Entries[1])
	}
	if len(requirements.RequiredModelFileSourceHashes) != 0 {
		t.Fatalf("expected empty source hashes, got %v", requirements.RequiredModelFileSourceHashes)
	}
}
