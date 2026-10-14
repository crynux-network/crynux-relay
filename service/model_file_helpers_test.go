package service

import (
	"crynux_relay/models"
	"testing"

	"gorm.io/gorm"
)

func testModelFileEntry(repository, revision string) models.ModelFileEntry {
	return models.ModelFileEntry{
		SourceType:   models.ModelSourceTypeHuggingFace,
		Repository:   repository,
		Revision:     revision,
		FormatKwargs: map[string]any{},
	}
}

func testModelFile(t testing.TB, repository, revision string) models.ModelFile {
	t.Helper()
	file, err := models.NewModelFile(testModelFileEntry(repository, revision))
	if err != nil {
		t.Fatalf("failed to build model file: %v", err)
	}
	return *file
}

func testModelFileHash(t testing.TB, repository, revision string) models.Hash32 {
	t.Helper()
	hash, err := testModelFileEntry(repository, revision).ModelFileHash()
	if err != nil {
		t.Fatalf("failed to hash model file: %v", err)
	}
	return hash
}

func testModelFileSourceHash(t testing.TB, repository string) models.Hash32 {
	t.Helper()
	hash, err := testModelFileEntry(repository, "").ModelFileSourceHash()
	if err != nil {
		t.Fatalf("failed to hash model file source: %v", err)
	}
	return hash
}

func migrateModelStateTables(t testing.TB, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&models.ModelFile{},
		&models.NodeModel{},
		&models.NodeInMemoryModel{},
		&models.NodeModelDownloadSelection{},
	); err != nil {
		t.Fatalf("failed to migrate model state tables: %v", err)
	}
}

// seedNodeModelFiles records the files in the catalog and in the node disk inventory.
func seedNodeModelFiles(t testing.TB, db *gorm.DB, address string, files ...models.ModelFile) {
	t.Helper()
	if err := models.UpsertModelFiles(t.Context(), db, files); err != nil {
		t.Fatalf("failed to upsert model files: %v", err)
	}
	for _, file := range files {
		hash, err := models.Hash32FromBytes(file.ModelFileHash)
		if err != nil {
			t.Fatalf("invalid model file hash: %v", err)
		}
		nodeModel := models.NewNodeModel(address, hash)
		if err := db.Create(&nodeModel).Error; err != nil {
			t.Fatalf("failed to seed node model: %v", err)
		}
	}
}

func llmTaskArgs(model, revision string) string {
	args := `{"version":"3.7.0","model":"` + model + `","dtype":"auto"`
	if revision != "" {
		args += `,"revision":"` + revision + `"`
	}
	return args + `,"messages":[{"role":"user","content":"hi"}]}`
}

func imageTaskArgs(model, revision string) string {
	base := `{"name":"` + model + `"`
	if revision != "" {
		base += `,"revision":"` + revision + `"`
	}
	return `{"version":"3.7.0","base_model":` + base + `},"dtype":"float16","prompt":"a cat"}`
}

// setTaskModelRequirements derives the model requirement columns of the task
// from its TaskArgs.
func setTaskModelRequirements(t testing.TB, task *models.InferenceTask) *models.TaskModelRequirements {
	t.Helper()
	requirements, err := models.DeriveTaskModelRequirements(task.TaskArgs, task.TaskType)
	if err != nil {
		t.Fatalf("failed to derive task model requirements: %v", err)
	}
	task.SetModelRequirements(requirements)
	return requirements
}

// applyTaskModelRequirements sets the model requirement columns of the task
// and records its exact model files in the catalog.
func applyTaskModelRequirements(t testing.TB, db *gorm.DB, task *models.InferenceTask) {
	t.Helper()
	requirements := setTaskModelRequirements(t, task)
	files, err := requirements.ModelFiles()
	if err != nil {
		t.Fatalf("failed to build model files: %v", err)
	}
	if err := models.UpsertModelFiles(t.Context(), db, files); err != nil {
		t.Fatalf("failed to upsert model files: %v", err)
	}
}

// registerTaskModelColumnDefaults stores empty model requirement columns for
// tasks that a test creates without model requirements.
func registerTaskModelColumnDefaults(db *gorm.DB) {
	err := db.Callback().Create().Before("gorm:before_create").Register("test:task_model_column_defaults", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "InferenceTask" {
			return
		}
		fill := func(task *models.InferenceTask) {
			if task.RequiredModelFileHashes == nil {
				task.RequiredModelFileHashes = []byte{}
			}
			if task.RequiredModelFileSourceHashes == nil {
				task.RequiredModelFileSourceHashes = []byte{}
			}
			if task.RequestedModelCacheHash == nil {
				task.RequestedModelCacheHash = models.Hash32{}.Bytes()
			}
			if task.QuantizationConfigHash == nil {
				task.QuantizationConfigHash = models.Hash32{}.Bytes()
			}
		}
		switch value := tx.Statement.Dest.(type) {
		case *models.InferenceTask:
			fill(value)
		case *[]models.InferenceTask:
			for i := range *value {
				fill(&(*value)[i])
			}
		case []models.InferenceTask:
			for i := range value {
				fill(&value[i])
			}
		}
	})
	if err != nil {
		panic(err)
	}
}

func testImageBaseModelFileEntry(repository, revision string) models.ModelFileEntry {
	entry := testModelFileEntry(repository, revision)
	entry.Format = models.ImageBaseModelFormat
	return entry
}

func testImageBaseModelFile(t testing.TB, repository, revision string) models.ModelFile {
	t.Helper()
	file, err := models.NewModelFile(testImageBaseModelFileEntry(repository, revision))
	if err != nil {
		t.Fatalf("failed to build model file: %v", err)
	}
	return *file
}

func testImageBaseModelFileHash(t testing.TB, repository, revision string) models.Hash32 {
	t.Helper()
	hash, err := testImageBaseModelFileEntry(repository, revision).ModelFileHash()
	if err != nil {
		t.Fatalf("failed to hash model file: %v", err)
	}
	return hash
}
