package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"database/sql"
	"math"
	"math/big"
	"testing"
	"time"
)

// initMatchingTestConfig initializes config plus the delegation cache for the
// empty test network so that weight calculation can run.
func initMatchingTestConfig(t *testing.T) {
	t.Helper()
	initServiceTestConfig(t)
	globalDelegationCaches = map[string]*delegationCache{
		"": {
			nodeDelegations: make(map[string]map[string]*big.Int),
			userDelegations: make(map[string]map[string]*big.Int),
			userStakeAmount: make(map[string]*big.Int),
			nodeStakeAmount: make(map[string]*big.Int),
		},
	}
	UpdateMaxStaking("0xseed", big.NewInt(100))
}

func newMatchingTestEntry(address string) *NodeIndexEntry {
	return &NodeIndexEntry{
		Address:                      address,
		Status:                       models.NodeStatusAvailable,
		GPUName:                      "NVIDIA GeForce RTX 4090",
		GPUVram:                      24,
		MajorVersion:                 3,
		MinorVersion:                 1,
		PatchVersion:                 0,
		QOSScore:                     float64(TASK_SCORE_REWARDS[0]),
		HealthBase:                   1.0,
		StakeAmount:                  big.NewInt(1),
		OnDiskModelFileHashes:        map[models.Hash32]struct{}{},
		OnDiskModelFileSourceHashes:  map[models.Hash32]struct{}{},
		CurrentModelFileHashes:       map[models.Hash32]struct{}{},
		CurrentModelFileSourceHashes: map[models.Hash32]struct{}{},
	}
}

func newMatchingTestTask(t *testing.T) *models.InferenceTask {
	t.Helper()
	task := &models.InferenceTask{
		TaskType:    models.TaskTypeSD,
		TaskVersion: "3.0.0",
		MinVRAM:     8,
		TaskArgs:    imageTaskArgs("model-a", "rev-a"),
	}
	setTaskModelRequirements(t, task)
	return task
}

var matchingVAEEntry = models.ModelFileEntry{
	SourceType: models.ModelSourceTypeHuggingFace, Repository: "org/vae", Revision: "rev-v", Format: "vae",
}

func matchingFileEntry(repository, revision string) models.ModelFileEntry {
	if repository == matchingVAEEntry.Repository {
		entry := matchingVAEEntry
		entry.Revision = revision
		return entry
	}
	return testImageBaseModelFileEntry(repository, revision)
}

func holdOnDisk(t *testing.T, entry *NodeIndexEntry, repository, revision string) {
	t.Helper()
	file := matchingFileEntry(repository, revision)
	hash, err := file.ModelFileHash()
	if err != nil {
		t.Fatalf("hash model file: %v", err)
	}
	sourceHash, err := file.ModelFileSourceHash()
	if err != nil {
		t.Fatalf("hash model file source: %v", err)
	}
	entry.OnDiskModelFileHashes[hash] = struct{}{}
	entry.OnDiskModelFileSourceHashes[sourceHash] = struct{}{}
}

func holdInMemory(t *testing.T, entry *NodeIndexEntry, task *models.InferenceTask, repository, revision string) {
	t.Helper()
	cacheHash, err := task.RequestedModelCacheHash32()
	if err != nil {
		t.Fatalf("decode cache hash: %v", err)
	}
	file := matchingFileEntry(repository, revision)
	hash, err := file.ModelFileHash()
	if err != nil {
		t.Fatalf("hash model file: %v", err)
	}
	sourceHash, err := file.ModelFileSourceHash()
	if err != nil {
		t.Fatalf("hash model file source: %v", err)
	}
	entry.CurrentModelCacheHash = &cacheHash
	entry.CurrentModelFileHashes[hash] = struct{}{}
	entry.CurrentModelFileSourceHashes[sourceHash] = struct{}{}
}
func TestFilterIndexEntriesForTaskHardFilters(t *testing.T) {
	task := newMatchingTestTask(t)

	available := newMatchingTestEntry("0xavailable")
	busy := newMatchingTestEntry("0xbusy")
	busy.Status = models.NodeStatusBusy
	occupied := newMatchingTestEntry("0xoccupied")
	occupied.HasCurrentTask = true
	lowVram := newMatchingTestEntry("0xlowvram")
	lowVram.GPUVram = 4
	oldVersion := newMatchingTestEntry("0xoldversion")
	oldVersion.MajorVersion = 2

	filtered := filterIndexEntriesForTask(task, []*NodeIndexEntry{available, busy, occupied, lowVram, oldVersion})
	if len(filtered) != 1 || filtered[0].Address != "0xavailable" {
		t.Fatalf("expected only the available node to pass hard filters, got %d entries", len(filtered))
	}
}

func TestFilterIndexEntriesForTaskRequiresNodeMinorVersion(t *testing.T) {
	task := newMatchingTestTask(t)
	task.TaskVersion = "3.6.0"

	current := newMatchingTestEntry("0xcurrent")
	current.MinorVersion = 6
	old := newMatchingTestEntry("0xold")
	old.MinorVersion = 5

	filtered := filterIndexEntriesForTask(task, []*NodeIndexEntry{current, old})
	if len(filtered) != 1 || filtered[0].Address != current.Address {
		t.Fatalf("expected only version 3.6.0 node, got %+v", filtered)
	}
}

func TestFilterIndexEntriesForTaskRequiredGPU(t *testing.T) {
	task := newMatchingTestTask(t)
	task.RequiredGPU = "NVIDIA A100"
	task.RequiredGPUVRAM = 40

	matching := newMatchingTestEntry("0xmatch")
	matching.GPUName = "NVIDIA A100"
	matching.GPUVram = 40
	wrongName := newMatchingTestEntry("0xwrongname")
	wrongVram := newMatchingTestEntry("0xwrongvram")
	wrongVram.GPUName = "NVIDIA A100"
	wrongVram.GPUVram = 80

	filtered := filterIndexEntriesForTask(task, []*NodeIndexEntry{matching, wrongName, wrongVram})
	if len(filtered) != 1 || filtered[0].Address != "0xmatch" {
		t.Fatalf("expected only the exact GPU match, got %d entries", len(filtered))
	}
}

func TestFilterIndexEntriesForTaskExcludesDarwinForLLM(t *testing.T) {
	task := newMatchingTestTask(t)
	task.TaskType = models.TaskTypeLLM

	linux := newMatchingTestEntry("0xlinux")
	linux.GPUName = "NVIDIA GeForce RTX 4090+Linux"
	darwin := newMatchingTestEntry("0xdarwin")
	darwin.GPUName = "Apple M3+Darwin"
	bare := newMatchingTestEntry("0xbare")

	filtered := filterIndexEntriesForTask(task, []*NodeIndexEntry{linux, darwin, bare})
	if len(filtered) != 1 || filtered[0].Address != "0xlinux" {
		t.Fatalf("expected only the non-Darwin node for LLM, got %d entries", len(filtered))
	}
}

func TestFilterIndexEntriesForTaskRequireLinuxCuda(t *testing.T) {
	task := newMatchingTestTask(t)
	task.RequireLinuxCuda = true
	task.TaskType = models.TaskTypeSD

	linux := newMatchingTestEntry("0xlinux")
	linux.GPUName = "NVIDIA GeForce RTX 4090+Linux"
	docker := newMatchingTestEntry("0xdocker")
	docker.GPUName = "NVIDIA GeForce RTX 4090+docker"
	windows := newMatchingTestEntry("0xwindows")
	windows.GPUName = "NVIDIA GeForce RTX 4090+Windows"
	darwin := newMatchingTestEntry("0xdarwin")
	darwin.GPUName = "Apple M3+Darwin"
	bare := newMatchingTestEntry("0xbare")

	filtered := filterIndexEntriesForTask(task, []*NodeIndexEntry{linux, docker, windows, darwin, bare})
	if len(filtered) != 2 {
		t.Fatalf("expected linux and docker only, got %d", len(filtered))
	}
	addresses := map[string]bool{filtered[0].Address: true, filtered[1].Address: true}
	if !addresses["0xlinux"] || !addresses["0xdocker"] {
		t.Fatalf("unexpected filtered set: %+v", filtered)
	}
}

func TestFilterIndexEntriesForTaskRequireLinuxCudaWithRequiredGPU(t *testing.T) {
	task := newMatchingTestTask(t)
	task.RequireLinuxCuda = true
	task.RequiredGPU = "NVIDIA GeForce RTX 4090+Windows"
	task.RequiredGPUVRAM = 24

	windows := newMatchingTestEntry("0xwindows")
	windows.GPUName = "NVIDIA GeForce RTX 4090+Windows"
	windows.GPUVram = 24
	linux := newMatchingTestEntry("0xlinux")
	linux.GPUName = "NVIDIA GeForce RTX 4090+Linux"
	linux.GPUVram = 24

	filtered := filterIndexEntriesForTask(task, []*NodeIndexEntry{windows, linux})
	if len(filtered) != 0 {
		t.Fatalf("expected empty candidates when RequiredGPU is Windows under RequireLinuxCuda, got %d", len(filtered))
	}
}

func TestPlatformFromGPUNameAndLinuxCuda(t *testing.T) {
	if platformFromGPUName("GPU+Linux") != "Linux" {
		t.Fatal("expected Linux platform")
	}
	if platformFromGPUName("GPU") != "" {
		t.Fatal("expected empty platform")
	}
	if !isLinuxCudaPlatform("A+docker") || !isLinuxCudaPlatform("A+Linux") {
		t.Fatal("expected docker and Linux to qualify")
	}
	if isLinuxCudaPlatform("A+Windows") || isLinuxCudaPlatform("A+Darwin") || isLinuxCudaPlatform("A") {
		t.Fatal("expected non-linux platforms to be rejected")
	}
}

func TestRequirementSignatureGroupsEqualRequirements(t *testing.T) {
	task1 := newMatchingTestTask(t)
	task2 := newMatchingTestTask(t)
	if requirementSignature(task1) != requirementSignature(task2) {
		t.Fatal("expected equal requirement signatures for identical requirements")
	}

	task2.MinVRAM = 24
	if requirementSignature(task1) == requirementSignature(task2) {
		t.Fatal("expected different signatures when MinVRAM differs")
	}

	task3 := newMatchingTestTask(t)
	task3.TaskArgs = imageTaskArgs("model-b", "rev-a")
	setTaskModelRequirements(t, task3)
	if requirementSignature(task1) == requirementSignature(task3) {
		t.Fatal("expected different signatures when model files differ")
	}

	task4 := newMatchingTestTask(t)
	task4.RequireLinuxCuda = true
	if requirementSignature(task1) == requirementSignature(task4) {
		t.Fatal("expected different signatures when RequireLinuxCuda differs")
	}
}

const matchingMultiFileTaskArgs = `{"version":"3.7.0","base_model":{"name":"model-a","revision":"rev-a"},"components":{"vae":{"format":"vae","repo":"org/vae","revision":"rev-v"}},"dtype":"float16","prompt":"a cat"}`

func TestBuildMatchingCandidateSetRequiresEveryExactFile(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask(t)
	task.TaskArgs = matchingMultiFileTaskArgs
	setTaskModelRequirements(t, task)
	withAll := newMatchingTestEntry("0xwithall")
	withAll.StakeAmount = big.NewInt(100)
	holdOnDisk(t, withAll, "model-a", "rev-a")
	holdOnDisk(t, withAll, "org/vae", "rev-v")
	withOne := newMatchingTestEntry("0xwithone")
	withOne.StakeAmount = big.NewInt(100)
	holdOnDisk(t, withOne, "model-a", "rev-a")
	otherRevision := newMatchingTestEntry("0xotherrevision")
	otherRevision.StakeAmount = big.NewInt(100)
	holdOnDisk(t, otherRevision, "model-a", "rev-a")
	holdOnDisk(t, otherRevision, "org/vae", "rev-other")

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{withAll, withOne, otherRevision})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 1 || set.entries[0].Address != "0xwithall" {
		t.Fatalf("expected only the node with every exact file, got %d entries", len(set.entries))
	}
	if set.scores[0] != set.probs[0].ProbWeight {
		t.Fatalf("expected no locality weight without in-memory model, base %f, got %f", set.probs[0].ProbWeight, set.scores[0])
	}
}

func TestBuildMatchingCandidateSetSourceRequirementAcceptsAnyRevision(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask(t)
	task.TaskArgs = imageTaskArgs("model-a", "")
	setTaskModelRequirements(t, task)
	anyRevision := newMatchingTestEntry("0xany")
	anyRevision.StakeAmount = big.NewInt(100)
	holdOnDisk(t, anyRevision, "model-a", "rev-z")
	missing := newMatchingTestEntry("0xmissing")
	missing.StakeAmount = big.NewInt(100)
	holdOnDisk(t, missing, "model-b", "rev-a")

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{anyRevision, missing})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 1 || set.entries[0].Address != "0xany" {
		t.Fatalf("expected only the node holding some revision, got %d entries", len(set.entries))
	}
}

func TestBuildMatchingCandidateSetInMemoryLocalityWeight(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask(t)
	onDisk := newMatchingTestEntry("0xondisk")
	onDisk.StakeAmount = big.NewInt(100)
	holdOnDisk(t, onDisk, "model-a", "rev-a")
	inMemory := newMatchingTestEntry("0xinmemory")
	inMemory.StakeAmount = big.NewInt(100)
	holdOnDisk(t, inMemory, "model-a", "rev-a")
	holdInMemory(t, inMemory, task, "model-a", "rev-a")
	otherCache := newMatchingTestEntry("0xothercache")
	otherCache.StakeAmount = big.NewInt(100)
	holdOnDisk(t, otherCache, "model-a", "rev-a")
	holdInMemory(t, otherCache, task, "model-a", "rev-a")
	otherCacheHash := models.Hash32{0xff}
	otherCache.CurrentModelCacheHash = &otherCacheHash

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{onDisk, inMemory, otherCache})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 3 {
		t.Fatalf("expected all nodes in the candidate set, got %d", len(set.entries))
	}
	for i, entry := range set.entries {
		want := set.probs[i].ProbWeight
		if entry.Address == "0xinmemory" {
			want *= modelLocalityWeight
		}
		if math.Abs(set.scores[i]-want) > 1e-9 {
			t.Fatalf("unexpected score for %s: want %f, got %f", entry.Address, want, set.scores[i])
		}
	}
}
func TestBuildMatchingCandidateSetHardFiltersActiveHealthExclusion(t *testing.T) {
	initMatchingTestConfig(t)
	task := newMatchingTestTask(t)
	eligible := newMatchingTestEntry("0xeligible")
	holdOnDisk(t, eligible, "model-a", "rev-a")
	excluded := newMatchingTestEntry("0xexcluded")
	holdOnDisk(t, excluded, "model-a", "rev-a")
	excluded.HealthExcluded = true
	excluded.HealthBase = 0.5
	excluded.HealthUpdatedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{excluded, eligible})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 1 || set.entries[0].Address != eligible.Address {
		t.Fatalf("expected only eligible node, got %d candidates", len(set.entries))
	}
}

func TestBuildMatchingCandidateSetDoesNotFallbackWhenAllNodesHealthExcluded(t *testing.T) {
	initMatchingTestConfig(t)
	task := newMatchingTestTask(t)
	excluded := newMatchingTestEntry("0xexcluded")
	holdOnDisk(t, excluded, "model-a", "rev-a")
	excluded.HealthExcluded = true
	excluded.HealthBase = 0.5
	excluded.HealthUpdatedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{excluded})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 0 {
		t.Fatalf("expected no candidates, got %d", len(set.entries))
	}
}

func TestBuildMatchingCandidateSetIncludesRecoveredHealthExcludedNode(t *testing.T) {
	initMatchingTestConfig(t)
	task := newMatchingTestTask(t)
	recovered := newMatchingTestEntry("0xrecovered")
	holdOnDisk(t, recovered, "model-a", "rev-a")
	recovered.HealthExcluded = true
	recovered.HealthBase = config.GetConfig().QoS.HealthExcludeExitThreshold
	recovered.HealthUpdatedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{recovered})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 1 || set.entries[0].Address != recovered.Address {
		t.Fatalf("expected recovered node in candidate set, got %d candidates", len(set.entries))
	}
}

func TestMatchTaskFromCandidateSetRespectsReservations(t *testing.T) {
	task := newMatchingTestTask(t)
	entry1 := newMatchingTestEntry("0xnode1")
	entry2 := newMatchingTestEntry("0xnode2")
	set := &matchingCandidateSet{
		entries: []*NodeIndexEntry{entry1, entry2},
		scores:  []float64{0.5, 0.5},
		probs:   []NodeSelectingProb{{ProbWeight: 0.5}, {ProbWeight: 0.5}},
	}

	reserved := map[string]struct{}{"0xnode1": {}}
	emptyPoolCounts := make(map[metrics.SelectionLabels]int)
	pair := matchTaskFromCandidateSet(task, set, reserved, false, emptyPoolCounts)
	if pair == nil {
		t.Fatal("expected a match with one unreserved node")
	}
	if pair.nodeAddress != "0xnode2" {
		t.Fatalf("expected the unreserved node to be selected, got %s", pair.nodeAddress)
	}
	if len(emptyPoolCounts) != 0 {
		t.Fatalf("expected no empty pool counts after a successful match, got %v", emptyPoolCounts)
	}

	reserved["0xnode2"] = struct{}{}
	if pair := matchTaskFromCandidateSet(task, set, reserved, false, emptyPoolCounts); pair != nil {
		t.Fatal("expected no match when all candidates are reserved")
	}
	if total := len(emptyPoolCounts); total != 1 {
		t.Fatalf("expected one empty pool label tuple, got %d", total)
	}
	for _, count := range emptyPoolCounts {
		if count != 1 {
			t.Fatalf("expected empty pool count 1, got %d", count)
		}
	}
}

func TestFetchQueuedTaskPageOrdersByPriorityThenID(t *testing.T) {
	initServiceTestConfig(t)
	if err := config.GetDB().AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	db := config.GetDB()

	now := time.Now()
	tasks := []models.InferenceTask{
		{TaskIDCommitment: "0xlow", Status: models.TaskQueued, Priority: models.BigInt{Int: *big.NewInt(1)}, CreateTime: sql.NullTime{Time: now, Valid: true}},
		{TaskIDCommitment: "0xhigh", Status: models.TaskQueued, Priority: models.BigInt{Int: *big.NewInt(9)}, CreateTime: sql.NullTime{Time: now, Valid: true}},
		{TaskIDCommitment: "0xhigh2", Status: models.TaskQueued, Priority: models.BigInt{Int: *big.NewInt(9)}, CreateTime: sql.NullTime{Time: now, Valid: true}},
		{TaskIDCommitment: "0xstarted", Status: models.TaskStarted, Priority: models.BigInt{Int: *big.NewInt(9)}, CreateTime: sql.NullTime{Time: now, Valid: true}},
	}
	for i := range tasks {
		if err := db.Create(&tasks[i]).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	page, err := fetchQueuedTaskPage(context.Background(), nil, 0, 10)
	if err != nil {
		t.Fatalf("fetch queued task page: %v", err)
	}
	if len(page) != 3 {
		t.Fatalf("expected 3 queued tasks, got %d", len(page))
	}
	if page[0].TaskIDCommitment != "0xhigh" || page[1].TaskIDCommitment != "0xhigh2" || page[2].TaskIDCommitment != "0xlow" {
		t.Fatalf("unexpected order: %s, %s, %s", page[0].TaskIDCommitment, page[1].TaskIDCommitment, page[2].TaskIDCommitment)
	}

	nextPage, err := fetchQueuedTaskPage(context.Background(), &page[1].Priority.Int, page[1].ID, 10)
	if err != nil {
		t.Fatalf("fetch next queued task page: %v", err)
	}
	if len(nextPage) != 1 || nextPage[0].TaskIDCommitment != "0xlow" {
		t.Fatalf("expected pagination to continue after the last task, got %d tasks", len(nextPage))
	}
}
