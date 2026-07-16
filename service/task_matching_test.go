package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"database/sql"
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
		Address:        address,
		Status:         models.NodeStatusAvailable,
		GPUName:        "NVIDIA GeForce RTX 4090",
		GPUVram:        24,
		MajorVersion:   3,
		MinorVersion:   1,
		PatchVersion:   0,
		QOSScore:       float64(TASK_SCORE_REWARDS[0]),
		HealthBase:     1.0,
		StakeAmount:    big.NewInt(1),
		OnDiskModelIDs: map[string]struct{}{},
		InUseModelIDs:  map[string]struct{}{},
	}
}

func newMatchingTestTask() *models.InferenceTask {
	return &models.InferenceTask{
		TaskType:    models.TaskTypeSD,
		TaskVersion: "3.0.0",
		MinVRAM:     8,
		ModelIDs:    models.StringArray{"base:model-a"},
	}
}

func TestFilterIndexEntriesForTaskHardFilters(t *testing.T) {
	task := newMatchingTestTask()

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

func TestFilterIndexEntriesForTaskRequiredGPU(t *testing.T) {
	task := newMatchingTestTask()
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
	task := newMatchingTestTask()
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

func TestRequirementSignatureGroupsEqualRequirements(t *testing.T) {
	task1 := newMatchingTestTask()
	task2 := newMatchingTestTask()
	if requirementSignature(task1) != requirementSignature(task2) {
		t.Fatal("expected equal requirement signatures for identical requirements")
	}

	task2.MinVRAM = 24
	if requirementSignature(task1) == requirementSignature(task2) {
		t.Fatal("expected different signatures when MinVRAM differs")
	}

	task3 := newMatchingTestTask()
	task3.ModelIDs = models.StringArray{"base:model-b"}
	if requirementSignature(task1) == requirementSignature(task3) {
		t.Fatal("expected different signatures when model IDs differ")
	}
}

func TestBuildMatchingCandidateSetLocalityRestriction(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask()
	task.ModelIDs = models.StringArray{"base:model-a", "lora:adapter", "base:model-b"}
	withAllBaseModels := newMatchingTestEntry("0xwithall")
	withAllBaseModels.StakeAmount = big.NewInt(100)
	withAllBaseModels.OnDiskModelIDs["base:model-a"] = struct{}{}
	withAllBaseModels.OnDiskModelIDs["base:model-b"] = struct{}{}
	withOneBaseModel := newMatchingTestEntry("0xwithone")
	withOneBaseModel.StakeAmount = big.NewInt(100)
	withOneBaseModel.OnDiskModelIDs["base:model-a"] = struct{}{}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{withAllBaseModels, withOneBaseModel})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 1 || set.entries[0].Address != "0xwithall" {
		t.Fatalf("expected only the node with every base model, got %d entries", len(set.entries))
	}
	if set.scores[0] != set.probs[0].ProbWeight {
		t.Fatalf("expected no on-disk weight boost, base %f, got %f", set.probs[0].ProbWeight, set.scores[0])
	}
}

func TestBuildMatchingCandidateSetInMemoryBoostExceedsOnDisk(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask()
	onDisk := newMatchingTestEntry("0xondisk")
	onDisk.StakeAmount = big.NewInt(100)
	onDisk.OnDiskModelIDs["base:model-a"] = struct{}{}
	inMemory := newMatchingTestEntry("0xinmemory")
	inMemory.StakeAmount = big.NewInt(100)
	inMemory.OnDiskModelIDs["base:model-a"] = struct{}{}
	inMemory.InUseModelIDs["base:model-a"] = struct{}{}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{onDisk, inMemory})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 2 {
		t.Fatalf("expected both nodes in the candidate set, got %d", len(set.entries))
	}
	var onDiskScore, inMemoryScore float64
	for i, entry := range set.entries {
		if entry.Address == "0xondisk" {
			onDiskScore = set.scores[i]
		} else {
			inMemoryScore = set.scores[i]
		}
	}
	if inMemoryScore <= onDiskScore {
		t.Fatalf("expected in-memory boost %f to exceed base score %f", inMemoryScore, onDiskScore)
	}
}

func TestBuildMatchingCandidateSetIgnoresAuxiliaryModels(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask()
	task.ModelIDs = models.StringArray{"base:model-a", "lora:adapter"}
	entry := newMatchingTestEntry("0xnode")
	entry.OnDiskModelIDs["base:model-a"] = struct{}{}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{entry})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 1 {
		t.Fatalf("expected auxiliary model to be ignored by the hard gate, got %d candidates", len(set.entries))
	}
}

func TestBuildMatchingCandidateSetHasNoFallbackWithoutAllBaseModels(t *testing.T) {
	initMatchingTestConfig(t)

	task := newMatchingTestTask()
	task.ModelIDs = models.StringArray{"base:model-a", "base:model-b"}
	entry := newMatchingTestEntry("0xnode")
	entry.OnDiskModelIDs["base:model-a"] = struct{}{}

	set, err := buildMatchingCandidateSet(context.Background(), task, []*NodeIndexEntry{entry})
	if err != nil {
		t.Fatalf("build candidate set: %v", err)
	}
	if len(set.entries) != 0 {
		t.Fatalf("expected no base-ready candidates, got %d", len(set.entries))
	}
}

func TestMatchTaskFromCandidateSetRespectsReservations(t *testing.T) {
	task := newMatchingTestTask()
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
