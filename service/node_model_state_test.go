package service

import (
	"bytes"
	"database/sql"
	"errors"
	"testing"
	"time"

	"crynux_relay/config"
	"crynux_relay/models"

	"gorm.io/gorm"
)

func TestNodeStartTaskRejectsActiveHealthExclusion(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("failed to migrate node tables: %v", err)
	}
	node := models.Node{
		Address:         "0xexcluded-node",
		Status:          models.NodeStatusAvailable,
		HealthBase:      0.5,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	err := nodeStartTask(t.Context(), db, &node, "0xtask")
	if !errors.Is(err, ErrNodeHealthExcluded) {
		t.Fatalf("expected health exclusion error, got %v", err)
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusAvailable || stored.CurrentTaskIDCommitment.Valid || !stored.HealthExcluded {
		t.Fatal("expected rejected start to leave node available and excluded")
	}
}

func TestNodeStartTaskClearsRecoveredExclusionAtomically(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("failed to migrate node tables: %v", err)
	}
	node := models.Node{
		Address:         "0xrecovered-node",
		Status:          models.NodeStatusAvailable,
		HealthBase:      config.GetConfig().QoS.HealthExcludeExitThreshold,
		HealthUpdatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:  true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := nodeStartTask(t.Context(), db, &node, "0xtask"); err != nil {
		t.Fatalf("start task: %v", err)
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusBusy ||
		!stored.CurrentTaskIDCommitment.Valid ||
		stored.CurrentTaskIDCommitment.String != "0xtask" ||
		stored.HealthExcluded {
		t.Fatal("expected busy state, current task, and exclusion clear to commit together")
	}
}

func TestNodeFinishTaskKeepsHealthExcludedNodeAvailable(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("failed to migrate node table: %v", err)
	}
	node := models.Node{
		Address:                 "0xfinished-excluded-node",
		Status:                  models.NodeStatusBusy,
		CurrentTaskIDCommitment: sql.NullString{String: "0xtask", Valid: true},
		HealthBase:              0.1,
		HealthUpdatedAt:         sql.NullTime{Time: time.Now().UTC(), Valid: true},
		HealthExcluded:          true,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := nodeFinishTask(t.Context(), db, &node); err != nil {
		t.Fatalf("finish task: %v", err)
	}
	var stored models.Node
	if err := db.First(&stored, node.ID).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.Status != models.NodeStatusAvailable || stored.CurrentTaskIDCommitment.Valid || !stored.HealthExcluded {
		t.Fatal("expected short-term exclusion to preserve Available status after task finish")
	}
}

func TestBuildHeldModelFilesValidatesAndDeduplicates(t *testing.T) {
	files, err := BuildHeldModelFiles([]models.ModelFileEntry{
		testModelFileEntry("org/model", "rev-1"),
		testModelFileEntry("org/model", "rev-1"),
		testModelFileEntry("org/model", "rev-2"),
	})
	if err != nil {
		t.Fatalf("build held model files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 distinct files, got %d", len(files))
	}

	invalid := []models.ModelFileEntry{
		testModelFileEntry("org/model", ""),
		testModelFileEntry("", "rev-1"),
		{SourceType: "other", Repository: "org/model", Revision: "rev-1"},
	}
	for _, entry := range invalid {
		if _, err := BuildHeldModelFiles([]models.ModelFileEntry{entry}); !errors.Is(err, ErrInvalidModelFile) {
			t.Fatalf("expected ErrInvalidModelFile for %+v, got %v", entry, err)
		}
	}
}

func newModelStateTestNode(t *testing.T) (*gorm.DB, models.Node) {
	t.Helper()
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatalf("failed to migrate node table: %v", err)
	}
	migrateModelStateTables(t, db)
	node := models.Node{Address: "0xmodel-node", Status: models.NodeStatusAvailable}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return db, node
}

func TestAddNodeModelFileIsIdempotent(t *testing.T) {
	db, node := newModelStateTestNode(t)
	file := testModelFile(t, "org/model", "rev-1")
	for i := 0; i < 2; i++ {
		if err := AddNodeModelFile(t.Context(), db, node.Address, file); err != nil {
			t.Fatalf("add node model file: %v", err)
		}
	}
	rows, err := models.ListNodeModelFileRows(t.Context(), db, node.Address)
	if err != nil || len(rows) != 1 {
		t.Fatalf("expected one inventory row, got %v, %v", rows, err)
	}

	if err := db.Model(&models.Node{}).Where("address = ?", node.Address).Update("status", models.NodeStatusQuit).Error; err != nil {
		t.Fatalf("quit node: %v", err)
	}
	if err := AddNodeModelFile(t.Context(), db, node.Address, file); !errors.Is(err, ErrNodeModelStateIllegal) {
		t.Fatalf("expected ErrNodeModelStateIllegal, got %v", err)
	}
}

func TestSetNodeInMemoryModelReplacesAndClears(t *testing.T) {
	db, node := newModelStateTestNode(t)
	file := testModelFile(t, "org/model", "rev-1")
	report := &NodeInMemoryModelReport{
		CacheHash:      models.Hash32{9},
		Files:          []models.ModelFile{file},
		CacheConfig:    []byte(`{"dtype":"auto"}`),
		ExecutionDType: "bfloat16",
	}
	if err := SetNodeInMemoryModel(t.Context(), db, node.Address, report); err != nil {
		t.Fatalf("set in-memory model: %v", err)
	}
	stored, err := models.GetNodeInMemoryModel(t.Context(), db, node.Address)
	if err != nil || stored == nil || stored.ExecutionDType != "bfloat16" {
		t.Fatalf("unexpected in-memory model: %+v, %v", stored, err)
	}
	_, inMemory, err := NodeModelFileEntries(t.Context(), db, node.Address)
	if err != nil || len(inMemory) != 1 || inMemory[0].Repository != "org/model" {
		t.Fatalf("unexpected in-memory entries: %+v, %v", inMemory, err)
	}

	if err := SetNodeInMemoryModel(t.Context(), db, node.Address, nil); err != nil {
		t.Fatalf("clear in-memory model: %v", err)
	}
	cleared, err := models.GetNodeInMemoryModel(t.Context(), db, node.Address)
	if err != nil || cleared != nil {
		t.Fatalf("expected cleared in-memory model, got %+v, %v", cleared, err)
	}
}

func TestClearNodeModelStateRemovesInventoryMemoryAndSelections(t *testing.T) {
	db, node := newModelStateTestNode(t)
	file := testModelFile(t, "org/model", "rev-1")
	seedNodeModelFiles(t, db, node.Address, file)
	hash := testModelFileHash(t, "org/model", "rev-1")
	memory := models.NewNodeInMemoryModel(node.Address, models.Hash32{1}, []models.Hash32{hash}, []byte(`{}`), "auto")
	if err := db.Create(&memory).Error; err != nil {
		t.Fatalf("create in-memory model: %v", err)
	}
	now := time.Now().UTC()
	selection := models.NewNodeModelDownloadSelection([]models.Hash32{hash}, nil, node.Address, 0, now, now.Add(time.Hour))
	if err := models.CreateNodeModelDownloadSelection(t.Context(), db, selection); err != nil {
		t.Fatalf("create selection: %v", err)
	}

	if err := clearNodeModelState(t.Context(), db, node.Address); err != nil {
		t.Fatalf("clear node model state: %v", err)
	}
	rows, err := models.ListNodeModelFileRows(t.Context(), db, node.Address)
	if err != nil || len(rows) != 0 {
		t.Fatalf("expected empty inventory, got %v, %v", rows, err)
	}
	cleared, err := models.GetNodeInMemoryModel(t.Context(), db, node.Address)
	if err != nil || cleared != nil {
		t.Fatalf("expected cleared in-memory model, got %+v, %v", cleared, err)
	}
	selections, err := models.GetAllNodeModelDownloadSelections(t.Context(), db)
	if err != nil || len(selections) != 0 {
		t.Fatalf("expected no selections, got %v, %v", selections, err)
	}
}

func fileRowOf(t *testing.T, repository, revision string) models.NodeModelFileRow {
	t.Helper()
	file := testModelFile(t, repository, revision)
	return models.NodeModelFileRow{ModelFileHash: file.ModelFileHash, ModelFileSourceHash: file.ModelFileSourceHash}
}

func TestResolveExecutionModelFileHashes(t *testing.T) {
	rowA := fileRowOf(t, "org/model", "rev-a")
	rowB := fileRowOf(t, "org/model", "rev-b")
	other := fileRowOf(t, "org/other", "rev-1")
	hashA := testModelFileHash(t, "org/model", "rev-a")
	hashB := testModelFileHash(t, "org/model", "rev-b")
	sourceHash := testModelFileSourceHash(t, "org/model")

	exact := &taskModelRequirements{exact: []models.Hash32{testModelFileHash(t, "org/other", "rev-1")}}
	resolved, err := resolveExecutionModelFileHashes(exact, []models.NodeModelFileRow{rowA, other}, nil)
	if err != nil || len(resolved) != 1 || resolved[0] != exact.exact[0] {
		t.Fatalf("unexpected exact resolution: %v, %v", resolved, err)
	}
	if _, err := resolveExecutionModelFileHashes(exact, []models.NodeModelFileRow{rowA}, nil); !errors.Is(err, errModelFileNotHeldForTask) {
		t.Fatalf("expected errModelFileNotHeldForTask, got %v", err)
	}

	source := &taskModelRequirements{source: []models.Hash32{sourceHash}}
	smallest := hashA
	if bytes.Compare(hashB.Bytes(), hashA.Bytes()) < 0 {
		smallest = hashB
	}
	resolved, err = resolveExecutionModelFileHashes(source, []models.NodeModelFileRow{rowA, rowB}, nil)
	if err != nil || len(resolved) != 1 || resolved[0] != smallest {
		t.Fatalf("expected smallest held revision, got %v, %v", resolved, err)
	}
	preferred := hashA
	if smallest == hashA {
		preferred = hashB
	}
	inMemory := map[models.Hash32]struct{}{preferred: {}}
	resolved, err = resolveExecutionModelFileHashes(source, []models.NodeModelFileRow{rowA, rowB}, inMemory)
	if err != nil || len(resolved) != 1 || resolved[0] != preferred {
		t.Fatalf("expected in-memory revision, got %v, %v", resolved, err)
	}
	if _, err := resolveExecutionModelFileHashes(source, []models.NodeModelFileRow{other}, nil); !errors.Is(err, errModelFileNotHeldForTask) {
		t.Fatalf("expected errModelFileNotHeldForTask, got %v", err)
	}
}

func TestNodeIndexEntryModelCoverage(t *testing.T) {
	hashA := testModelFileHash(t, "org/model", "rev-a")
	sourceHash := testModelFileSourceHash(t, "org/model")
	cacheHash := models.Hash32{5}
	entry := &NodeIndexEntry{
		OnDiskModelFileHashes:        map[models.Hash32]struct{}{hashA: {}},
		OnDiskModelFileSourceHashes:  map[models.Hash32]struct{}{sourceHash: {}},
		CurrentModelCacheHash:        &cacheHash,
		CurrentModelFileHashes:       map[models.Hash32]struct{}{hashA: {}},
		CurrentModelFileSourceHashes: map[models.Hash32]struct{}{sourceHash: {}},
	}

	exactReq := &taskModelRequirements{exact: []models.Hash32{hashA}, cacheHash: cacheHash}
	sourceReq := &taskModelRequirements{source: []models.Hash32{sourceHash}, cacheHash: cacheHash}
	missingReq := &taskModelRequirements{exact: []models.Hash32{testModelFileHash(t, "org/model", "rev-b")}, cacheHash: cacheHash}
	otherCache := &taskModelRequirements{exact: []models.Hash32{hashA}, cacheHash: models.Hash32{6}}

	if !entry.holdsRequiredModelFiles(exactReq) || !entry.holdsRequiredModelFiles(sourceReq) {
		t.Fatal("expected disk coverage of exact and source requirements")
	}
	if entry.holdsRequiredModelFiles(missingReq) {
		t.Fatal("expected a missing exact file to fail the disk gate")
	}
	if !entry.holdsRequiredModelInMemory(exactReq) || !entry.holdsRequiredModelInMemory(sourceReq) {
		t.Fatal("expected in-memory coverage")
	}
	if entry.holdsRequiredModelInMemory(otherCache) {
		t.Fatal("expected a different cache hash to miss the in-memory model")
	}
	entry.CurrentModelCacheHash = nil
	if entry.holdsRequiredModelInMemory(exactReq) {
		t.Fatal("expected empty memory to miss the in-memory model")
	}
}
