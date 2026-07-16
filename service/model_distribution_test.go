package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"math/big"
	"testing"
	"time"
)

func initModelDistributionTest(t *testing.T) {
	t.Helper()
	initMatchingTestConfig(t)
	if err := config.GetDB().AutoMigrate(
		&models.Node{},
		&models.NodeModel{},
		&models.InferenceTask{},
		&models.Event{},
		&models.NodeModelDownloadSelection{},
	); err != nil {
		t.Fatalf("failed to migrate model distribution tables: %v", err)
	}
}

func newDistributionTestNode(address string) models.Node {
	return models.Node{
		Address:     address,
		Status:      models.NodeStatusAvailable,
		GPUName:     "NVIDIA GeForce RTX 4090",
		GPUVram:     24,
		QOSScore:    float64(TASK_SCORE_REWARDS[0]),
		HealthBase:  1.0,
		StakeAmount: models.BigInt{Int: *big.NewInt(100)},
	}
}

func countDownloadModelEvents(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := config.GetDB().Model(&models.Event{}).Where("type = ?", "DownloadModel").Count(&count).Error; err != nil {
		t.Fatalf("count DownloadModel events: %v", err)
	}
	return count
}

func TestComputeTargetNodeCount(t *testing.T) {
	demand := &modelDemand{
		arrivalCount:        60,
		executionSecondsSum: 300,
		executionSecondsCnt: 3,
	}
	// arrival_rate = 60/1800, avg = 100, safety = 2.0 -> ceil(6.67) = 7
	if target := computeTargetNodeCount(demand, 1800, 2.0, 1, 10); target != 7 {
		t.Fatalf("expected target 7, got %d", target)
	}

	// Cold start: no completed tasks, fall back to the mean stored estimate.
	fallback := &modelDemand{
		arrivalCount:        60,
		estimatedSecondsSum: 200,
		estimatedSecondsCnt: 2,
	}
	if target := computeTargetNodeCount(fallback, 1800, 2.0, 1, 10); target != 7 {
		t.Fatalf("expected fallback target 7, got %d", target)
	}

	// Queued-only demand with no arrivals clamps up to min_nodes.
	queuedOnly := &modelDemand{queuedCount: 1}
	if target := computeTargetNodeCount(queuedOnly, 1800, 2.0, 2, 10); target != 2 {
		t.Fatalf("expected min clamp 2, got %d", target)
	}

	// Heavy demand clamps down to max_nodes.
	heavy := &modelDemand{
		arrivalCount:        18000,
		executionSecondsSum: 100,
		executionSecondsCnt: 1,
	}
	if target := computeTargetNodeCount(heavy, 1800, 2.0, 1, 10); target != 10 {
		t.Fatalf("expected max clamp 10, got %d", target)
	}
}

func TestSelectDownloadTargetNodesExclusions(t *testing.T) {
	initMatchingTestConfig(t)
	now := time.Now().UTC()

	holder := newDistributionTestNode("0xholder")
	pendingNode := newDistributionTestNode("0xpending")
	expiredNode := newDistributionTestNode("0xexpired")
	fresh := newDistributionTestNode("0xfresh")

	state := &modelSelectionState{
		attemptCounts: map[string]int{"0xpending": 1, "0xexpired": 1},
		nonExpired:    map[string]struct{}{"0xpending": {}},
	}
	holding := map[string]struct{}{"0xholder": {}}

	selected := selectDownloadTargetNodes(
		[]models.Node{holder, pendingNode, expiredNode, fresh},
		holding, state, now, 4,
	)
	if len(selected) != 1 || selected[0].Address != "0xfresh" {
		t.Fatalf("expected only the fresh node while unattempted nodes remain, got %v", selected)
	}
}

func TestSelectDownloadTargetNodesReadmitsExpiredWhenPoolEmpty(t *testing.T) {
	initMatchingTestConfig(t)
	now := time.Now().UTC()

	once := newDistributionTestNode("0xonce")
	twice := newDistributionTestNode("0xtwice")

	state := &modelSelectionState{
		attemptCounts: map[string]int{"0xonce": 1, "0xtwice": 2},
		nonExpired:    map[string]struct{}{},
	}

	selected := selectDownloadTargetNodes([]models.Node{twice, once}, nil, state, now, 1)
	if len(selected) != 1 || selected[0].Address != "0xonce" {
		t.Fatalf("expected the least-attempted node to be re-admitted first, got %v", selected)
	}

	selected = selectDownloadTargetNodes([]models.Node{twice, once}, nil, state, now, 2)
	if len(selected) != 2 || selected[0].Address != "0xonce" || selected[1].Address != "0xtwice" {
		t.Fatalf("expected re-admission in ascending attempt order, got %v", selected)
	}
}

func TestSampleDownloadTargetsZeroWeightLast(t *testing.T) {
	initMatchingTestConfig(t)
	now := time.Now().UTC()

	positive := newDistributionTestNode("0xzpositive")
	zeroB := newDistributionTestNode("0xb")
	zeroB.StakeAmount = models.BigInt{Int: *big.NewInt(0)}
	zeroA := newDistributionTestNode("0xa")
	zeroA.StakeAmount = models.BigInt{Int: *big.NewInt(0)}

	selected := sampleDownloadTargets([]models.Node{zeroB, positive, zeroA}, now, 3)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected nodes, got %d", len(selected))
	}
	if selected[0].Address != "0xzpositive" {
		t.Fatalf("expected the positive-weight node first, got %s", selected[0].Address)
	}
	if selected[1].Address != "0xa" || selected[2].Address != "0xb" {
		t.Fatalf("expected zero-weight nodes in ascending address order, got %s, %s", selected[1].Address, selected[2].Address)
	}
}

func TestApplySelectionStatusTransitions(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	completing := models.NewNodeModelDownloadSelection("base:model-a", "0xdone", 0, now.Add(-time.Minute), now.Add(time.Hour))
	expiring := models.NewNodeModelDownloadSelection("base:model-a", "0xlate", 0, now.Add(-2*time.Hour), now.Add(-time.Hour))
	waiting := models.NewNodeModelDownloadSelection("base:model-a", "0xwaiting", 0, now.Add(-time.Minute), now.Add(time.Hour))
	for _, selection := range []*models.NodeModelDownloadSelection{completing, expiring, waiting} {
		if err := models.CreateNodeModelDownloadSelection(ctx, db, selection); err != nil {
			t.Fatalf("create selection: %v", err)
		}
	}
	nodeModel := models.NewNodeModel("0xdone", "base:model-a", false)
	if err := nodeModel.Save(ctx, db); err != nil {
		t.Fatalf("create node model: %v", err)
	}

	if err := applySelectionStatusTransitions(ctx, db, now); err != nil {
		t.Fatalf("apply transitions: %v", err)
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	statuses := make(map[string]models.NodeModelDownloadSelectionStatus)
	actives := make(map[string]*bool)
	for _, selection := range selections {
		statuses[selection.NodeAddress] = selection.Status
		actives[selection.NodeAddress] = selection.Active
	}
	if statuses["0xdone"] != models.NodeModelDownloadSelectionCompleted {
		t.Fatalf("expected completed selection, got %s", statuses["0xdone"])
	}
	if statuses["0xlate"] != models.NodeModelDownloadSelectionExpired {
		t.Fatalf("expected expired selection, got %s", statuses["0xlate"])
	}
	if actives["0xlate"] != nil {
		t.Fatal("expected expired selection to clear the active marker")
	}
	if statuses["0xwaiting"] != models.NodeModelDownloadSelectionPending {
		t.Fatalf("expected pending selection to remain pending, got %s", statuses["0xwaiting"])
	}

	// A new attempt is allowed after expiry but duplicate non-expired records
	// are rejected by the uniqueness constraint.
	retry := models.NewNodeModelDownloadSelection("base:model-a", "0xlate", 0, now, now.Add(time.Hour))
	if err := models.CreateNodeModelDownloadSelection(ctx, db, retry); err != nil {
		t.Fatalf("create retry selection after expiry: %v", err)
	}
	duplicate := models.NewNodeModelDownloadSelection("base:model-a", "0xwaiting", 0, now, now.Add(time.Hour))
	if err := models.CreateNodeModelDownloadSelection(ctx, db, duplicate); err == nil {
		t.Fatal("expected duplicate non-expired selection to violate the unique index")
	}
}

func TestRunModelDistributionRoundEmitsAndConverges(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	node := newDistributionTestNode("0xcandidate")
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	task := models.InferenceTask{
		TaskIDCommitment:     "0xtask",
		Status:               models.TaskQueued,
		TaskType:             models.TaskTypeSD,
		ModelIDs:             models.StringArray{"base:model-a", "lora:adapter"},
		CreateTime:           sql.NullTime{Time: now, Valid: true},
		EstimatedNodeSeconds: 60,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("first round: %v", err)
	}
	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 1 {
		t.Fatalf("expected one selection, got %d", len(selections))
	}
	if selections[0].NodeAddress != node.Address || selections[0].ModelID != "base:model-a" ||
		selections[0].Status != models.NodeModelDownloadSelectionPending {
		t.Fatalf("unexpected selection: %+v", selections[0])
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected one DownloadModel event, got %d", count)
	}

	// A pending selection occupies capacity, so a second run emits nothing.
	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("second round: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected no re-send for a pending selection, got %d events", count)
	}

	// The node reports the model on disk: the selection completes and the
	// holding node covers the target, so still nothing new is emitted.
	nodeModel := models.NewNodeModel(node.Address, "base:model-a", false)
	if err := nodeModel.Save(ctx, db); err != nil {
		t.Fatalf("create node model: %v", err)
	}
	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("third round: %v", err)
	}
	selections, err = models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 1 || selections[0].Status != models.NodeModelDownloadSelectionCompleted {
		t.Fatalf("expected the selection to complete, got %+v", selections)
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected no new events at full coverage, got %d", count)
	}
}

func TestRunModelDistributionRoundRespectsTaskHardwareRequirement(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	small := newDistributionTestNode("0xsmall")
	big := newDistributionTestNode("0xbig")
	big.GPUName = "4x NVIDIA GeForce RTX 5090+docker"
	big.GPUVram = 128
	for _, node := range []*models.Node{&small, &big} {
		if err := db.Create(node).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
	}

	// The small node already holds the model, but it cannot execute the
	// demanding task, so it must not count toward the group's coverage.
	smallHolding := models.NewNodeModel(small.Address, "base:model-a", false)
	if err := smallHolding.Save(ctx, db); err != nil {
		t.Fatalf("create node model: %v", err)
	}

	task := models.InferenceTask{
		TaskIDCommitment:     "0xtask",
		Status:               models.TaskQueued,
		TaskType:             models.TaskTypeLLM,
		MinVRAM:              100,
		ModelIDs:             models.StringArray{"base:model-a"},
		CreateTime:           sql.NullTime{Time: now, Valid: true},
		EstimatedNodeSeconds: 60,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("round: %v", err)
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 1 {
		t.Fatalf("expected one selection for the qualified node, got %d", len(selections))
	}
	if selections[0].NodeAddress != big.Address {
		t.Fatalf("expected the qualified node to be selected, got %s", selections[0].NodeAddress)
	}

	// The qualified pending selection covers the group: no further emission.
	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("second round: %v", err)
	}
	if count := countDownloadModelEvents(t); count != 1 {
		t.Fatalf("expected exactly one DownloadModel event, got %d", count)
	}
}

func TestNodeSatisfiesDemand(t *testing.T) {
	llmDemand := modelDemandKey{modelID: "base:model-a", minVRAM: 100, excludeDarwin: true}
	if nodeSatisfiesDemand("NVIDIA RTX PRO 6000 Blackwell Server Edition+docker", 96, llmDemand) {
		t.Fatal("expected a node below min VRAM to be disqualified")
	}
	if !nodeSatisfiesDemand("4x NVIDIA GeForce RTX 5090+docker", 128, llmDemand) {
		t.Fatal("expected a node meeting min VRAM to qualify")
	}
	if nodeSatisfiesDemand("Apple M2 Ultra+Darwin", 192, llmDemand) {
		t.Fatal("expected a Darwin node to be disqualified for LLM demand")
	}
	if !nodeSatisfiesDemand("Apple M2 Ultra+Darwin", 192, modelDemandKey{modelID: "base:model-a", minVRAM: 100}) {
		t.Fatal("expected a Darwin node to qualify without the LLM exclusion")
	}
}

func TestTaskVRAMDemand(t *testing.T) {
	task := &models.InferenceTask{MinVRAM: 100}
	if taskVRAMDemand(task) != 100 {
		t.Fatalf("expected min VRAM demand 100, got %d", taskVRAMDemand(task))
	}
	pinned := &models.InferenceTask{MinVRAM: 8, RequiredGPU: "NVIDIA A100", RequiredGPUVRAM: 40}
	if taskVRAMDemand(pinned) != 40 {
		t.Fatalf("expected pinned GPU VRAM demand 40, got %d", taskVRAMDemand(pinned))
	}
}

func TestRunModelDistributionRoundReplacesExpiredSelection(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	first := newDistributionTestNode("0xfirst")
	second := newDistributionTestNode("0xsecond")
	for _, node := range []*models.Node{&first, &second} {
		if err := db.Create(node).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
	}
	task := models.InferenceTask{
		TaskIDCommitment:     "0xtask",
		Status:               models.TaskQueued,
		TaskType:             models.TaskTypeSD,
		ModelIDs:             models.StringArray{"base:model-a"},
		CreateTime:           sql.NullTime{Time: now, Valid: true},
		EstimatedNodeSeconds: 60,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	expired := models.NewNodeModelDownloadSelection("base:model-a", first.Address, 0, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err := models.CreateNodeModelDownloadSelection(ctx, db, expired); err != nil {
		t.Fatalf("create expired selection: %v", err)
	}

	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("round: %v", err)
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 2 {
		t.Fatalf("expected the expired selection plus one replacement, got %d", len(selections))
	}
	var replacement *models.NodeModelDownloadSelection
	for i := range selections {
		if selections[i].Status == models.NodeModelDownloadSelectionPending {
			replacement = &selections[i]
		}
	}
	if replacement == nil {
		t.Fatal("expected a pending replacement selection")
	}
	if replacement.NodeAddress != second.Address {
		t.Fatalf("expected the unattempted node to be selected, got %s", replacement.NodeAddress)
	}
}

func TestRunModelDistributionRoundCleansUpWithoutDemand(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	selection := models.NewNodeModelDownloadSelection("base:model-a", "0xnode", 0, now, now.Add(time.Hour))
	if err := models.CreateNodeModelDownloadSelection(ctx, db, selection); err != nil {
		t.Fatalf("create selection: %v", err)
	}

	if err := runModelDistributionRound(ctx, db); err != nil {
		t.Fatalf("round: %v", err)
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 0 {
		t.Fatalf("expected demand-period cleanup to delete selections, got %d", len(selections))
	}
}

func TestDeleteNodeModelDownloadSelectionsByNodeAddress(t *testing.T) {
	initModelDistributionTest(t)
	db := config.GetDB()
	ctx := context.Background()
	now := time.Now().UTC()

	mine := models.NewNodeModelDownloadSelection("base:model-a", "0xquitting", 0, now, now.Add(time.Hour))
	other := models.NewNodeModelDownloadSelection("base:model-a", "0xstaying", 0, now, now.Add(time.Hour))
	for _, selection := range []*models.NodeModelDownloadSelection{mine, other} {
		if err := models.CreateNodeModelDownloadSelection(ctx, db, selection); err != nil {
			t.Fatalf("create selection: %v", err)
		}
	}

	if err := models.DeleteNodeModelDownloadSelectionsByNodeAddress(ctx, db, "0xquitting"); err != nil {
		t.Fatalf("delete selections: %v", err)
	}
	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		t.Fatalf("load selections: %v", err)
	}
	if len(selections) != 1 || selections[0].NodeAddress != "0xstaying" {
		t.Fatalf("expected only the other node's selection to remain, got %+v", selections)
	}
}
