package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"fmt"
	"math"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/stat/sampleuv"
	"gorm.io/gorm"
)

// modelDemandKey identifies one distribution unit: a required model file set
// together with the VRAM demand, the LLM Darwin exclusion, and RequireLinuxCuda
// of the tasks requiring it. Tasks with different VRAM demands form separate
// demand groups because a node only counts toward coverage of the groups it
// can execute.
type modelDemandKey struct {
	setKey           models.Hash32
	minVRAM          uint64
	excludeDarwin    bool
	requireLinuxCuda bool
}

// taskVRAMDemand is the VRAM amount a task requires from a node: the exact
// GPU VRAM when the task pins a GPU, otherwise the minimum VRAM bound.
func taskVRAMDemand(task *models.InferenceTask) uint64 {
	if len(task.RequiredGPU) > 0 {
		return task.RequiredGPUVRAM
	}
	return task.MinVRAM
}

// modelDemandKeyOf returns the demand group of a task and its required model
// file hashes. ok is false when the task requires no model files.
func modelDemandKeyOf(task *models.InferenceTask) (key modelDemandKey, exact, source []models.Hash32, ok bool, err error) {
	exact, err = task.RequiredModelFileHashList()
	if err != nil {
		return key, nil, nil, false, err
	}
	source, err = task.RequiredModelFileSourceHashList()
	if err != nil {
		return key, nil, nil, false, err
	}
	if len(exact) == 0 && len(source) == 0 {
		return key, nil, nil, false, nil
	}
	key = modelDemandKey{
		setKey:           models.DistributionKey(exact, source),
		minVRAM:          taskVRAMDemand(task),
		excludeDarwin:    task.TaskType == models.TaskTypeLLM,
		requireLinuxCuda: task.RequireLinuxCuda,
	}
	return key, exact, source, true, nil
}

// nodeSatisfiesDemand reports whether a node's VRAM covers the demand
// group's requirement, the node is not a Darwin node for LLM demand, and the
// node is a Linux CUDA platform when RequireLinuxCuda is set.
func nodeSatisfiesDemand(gpuName string, gpuVram uint64, key modelDemandKey) bool {
	if gpuVram < key.minVRAM {
		return false
	}
	if key.excludeDarwin && isDarwinGPUName(gpuName) {
		return false
	}
	if key.requireLinuxCuda && !isLinuxCudaPlatform(gpuName) {
		return false
	}
	return true
}

// modelDemand aggregates the demand facts of one demand group over the
// current demand window: queued tasks, window arrivals, measured execution
// durations and the pricing estimates used as the cold-start fallback.
type modelDemand struct {
	key                 modelDemandKey
	exact               []models.Hash32
	source              []models.Hash32
	representativeTask  uint
	queuedCount         int
	arrivalCount        int
	latestCreateTime    time.Time
	latestTaskType      models.TaskType
	estimatedSecondsSum float64
	estimatedSecondsCnt int
	executionSecondsSum float64
	executionSecondsCnt int
}

// modelSelectionState is the per-model-set view over the persisted selection
// records used for capacity accounting and pool exclusion.
type modelSelectionState struct {
	// attemptCounts counts all selection records per node address.
	attemptCounts map[string]int
	// nonExpired marks nodes with a pending or completed record.
	nonExpired map[string]struct{}
	// pendingNodes marks nodes with a pending record.
	pendingNodes map[string]struct{}
}

func newModelSelectionState() *modelSelectionState {
	return &modelSelectionState{
		attemptCounts: make(map[string]int),
		nonExpired:    make(map[string]struct{}),
		pendingNodes:  make(map[string]struct{}),
	}
}

func StartModelDistribution(ctx context.Context, db *gorm.DB) {
	interval := time.Duration(config.GetConfig().ModelDistribution.ControllerIntervalSeconds * float64(time.Second))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runModelDistributionRound(ctx, db); err != nil {
				log.Errorf("ModelDistribution: controller round error: %v", err)
			}
		}
	}
}

func runModelDistributionRound(ctx context.Context, db *gorm.DB) error {
	cfg := config.GetConfig().ModelDistribution
	now := time.Now().UTC()
	windowStart := now.Add(-time.Duration(cfg.DemandWindowSeconds * float64(time.Second)))

	if err := applySelectionStatusTransitions(ctx, db, now); err != nil {
		return err
	}

	demands, err := collectModelDemands(ctx, db, windowStart)
	if err != nil {
		return err
	}

	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		return err
	}
	selectionStates, err := buildModelSelectionStates(selections)
	if err != nil {
		return err
	}

	if err := cleanupSelectionsWithoutDemand(ctx, db, selectionStates, demands); err != nil {
		return err
	}

	if len(demands) == 0 {
		return nil
	}

	candidateNodes, err := getDownloadCandidateNodes(ctx, db)
	if err != nil {
		return err
	}
	holdingNodes, err := getModelHoldingNodes(ctx, db, demands, candidateNodes)
	if err != nil {
		return err
	}
	nodeHardwareByAddress := make(map[string]models.Node, len(candidateNodes))
	for _, node := range candidateNodes {
		nodeHardwareByAddress[node.Address] = node
	}
	candidateNodes, err = filterNodesByNodeNamePolicy(ctx, candidateNodes)
	if err != nil {
		return err
	}

	for _, demand := range demands {
		setKey := demand.key.setKey
		state := selectionStates[setKey]
		target := computeTargetNodeCount(demand, cfg.DemandWindowSeconds, cfg.SafetyFactor, cfg.MinNodes, cfg.MaxNodes)

		// Coverage and pending capacity count only nodes that satisfy the
		// demand group's VRAM requirement; a copy on a node that cannot
		// execute the demanding tasks does not serve them.
		holding := holdingNodes[demand.key]
		qualifiedHoldingCount := 0
		for address := range holding {
			if node, ok := nodeHardwareByAddress[address]; ok && nodeSatisfiesDemand(node.GPUName, node.GPUVram, demand.key) {
				qualifiedHoldingCount++
			}
		}
		qualifiedPendingCount := 0
		if state != nil {
			for address := range state.pendingNodes {
				if node, ok := nodeHardwareByAddress[address]; ok && nodeSatisfiesDemand(node.GPUName, node.GPUVram, demand.key) {
					qualifiedPendingCount++
				}
			}
		}

		deficit := target - qualifiedHoldingCount - qualifiedPendingCount
		if deficit <= 0 {
			continue
		}

		qualifiedCandidates := make([]models.Node, 0, len(candidateNodes))
		for _, node := range candidateNodes {
			if nodeSatisfiesDemand(node.GPUName, node.GPUVram, demand.key) {
				qualifiedCandidates = append(qualifiedCandidates, node)
			}
		}

		selected := selectDownloadTargetNodes(qualifiedCandidates, holding, state, now, deficit)
		if len(selected) == 0 {
			continue
		}
		modelFiles, err := loadDemandModelFiles(ctx, db, demand)
		if err != nil {
			log.Errorf("ModelDistribution: load model files of set %s error: %v", setKey.Hex(), err)
			continue
		}
		for _, node := range selected {
			if err := emitModelDownloadSelection(ctx, db, demand, modelFiles, node.Address, now, cfg.DownloadTimeoutSeconds); err != nil {
				log.Errorf("ModelDistribution: emit download selection for set %s on node %s error: %v", setKey.Hex(), node.Address, err)
				continue
			}
			// Record the new selection in the in-memory state so later
			// demand groups of the same set see it as pending capacity
			// and do not select the node again in this round.
			if state == nil {
				state = newModelSelectionState()
				selectionStates[setKey] = state
			}
			state.attemptCounts[node.Address]++
			state.nonExpired[node.Address] = struct{}{}
			state.pendingNodes[node.Address] = struct{}{}
		}
	}
	return nil
}

// applySelectionStatusTransitions moves pending selections to completed when
// the node's authoritative on-disk inventory covers the required model file
// set, and to expired when the deadline has passed without completion.
func applySelectionStatusTransitions(ctx context.Context, db *gorm.DB, now time.Time) error {
	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		return err
	}

	completedIDSet := make(map[uint]struct{})
	hasPending := false
	for _, selection := range selections {
		if selection.Status == models.NodeModelDownloadSelectionPending {
			hasPending = true
			break
		}
	}
	if hasPending {
		rows, err := models.ListNodeModelFileRows(ctx, db)
		if err != nil {
			return err
		}
		inventories, err := nodeFileSetsByAddress(rows)
		if err != nil {
			return err
		}
		completedIDs := make([]uint, 0)
		for i := range selections {
			selection := &selections[i]
			if selection.Status != models.NodeModelDownloadSelectionPending {
				continue
			}
			exact, source, err := selection.RequiredHashes()
			if err != nil {
				return err
			}
			if inventory, ok := inventories[selection.NodeAddress]; ok && modelFilesCovered(inventory.exact, inventory.source, exact, source) {
				completedIDs = append(completedIDs, selection.ID)
			}
		}
		if err := models.CompleteNodeModelDownloadSelections(ctx, db, completedIDs); err != nil {
			return err
		}
		for _, id := range completedIDs {
			completedIDSet[id] = struct{}{}
		}
		for _, selection := range selections {
			if _, ok := completedIDSet[selection.ID]; ok {
				metrics.ModelDownloadsCompleted.WithLabelValues(metrics.VramTierLabel(selection.MinVRAM)).Inc()
			}
		}
	}

	if err := models.ExpireNodeModelDownloadSelections(ctx, db, now); err != nil {
		return err
	}
	for _, selection := range selections {
		if selection.Status != models.NodeModelDownloadSelectionPending {
			continue
		}
		if _, ok := completedIDSet[selection.ID]; ok {
			continue
		}
		if selection.Deadline.Before(now) {
			metrics.ModelDownloadsExpired.WithLabelValues(metrics.VramTierLabel(selection.MinVRAM)).Inc()
		}
	}
	return nil
}

// nodeFileSets is the disk inventory of one node as exact and source hash sets.
type nodeFileSets struct {
	exact  map[models.Hash32]struct{}
	source map[models.Hash32]struct{}
}

func nodeFileSetsByAddress(rows []models.NodeModelFileRow) (map[string]*nodeFileSets, error) {
	inventories := make(map[string]*nodeFileSets)
	for _, row := range rows {
		hash, err := models.Hash32FromBytes(row.ModelFileHash)
		if err != nil {
			return nil, err
		}
		sourceHash, err := models.Hash32FromBytes(row.ModelFileSourceHash)
		if err != nil {
			return nil, err
		}
		inventory, ok := inventories[row.NodeAddress]
		if !ok {
			inventory = &nodeFileSets{
				exact:  make(map[models.Hash32]struct{}),
				source: make(map[models.Hash32]struct{}),
			}
			inventories[row.NodeAddress] = inventory
		}
		inventory.exact[hash] = struct{}{}
		inventory.source[sourceHash] = struct{}{}
	}
	return inventories, nil
}

// collectModelDemands measures demand per demand group (required model file
// set plus task VRAM demand) from tasks that are currently queued or were
// created within the demand window, plus execution durations of tasks
// completed within the window.
func collectModelDemands(ctx context.Context, db *gorm.DB, windowStart time.Time) (map[modelDemandKey]*modelDemand, error) {
	demands := make(map[modelDemandKey]*modelDemand)

	demandTasks, err := getDemandTasks(ctx, db, windowStart)
	if err != nil {
		return nil, err
	}
	for i := range demandTasks {
		task := &demandTasks[i]
		key, exact, source, ok, err := modelDemandKeyOf(task)
		if err != nil {
			return nil, fmt.Errorf("task %d required model files: %w", task.ID, err)
		}
		if !ok {
			continue
		}
		demand, ok := demands[key]
		if !ok {
			demand = &modelDemand{key: key, exact: exact, source: source, representativeTask: task.ID}
			demands[key] = demand
		}
		if task.Status == models.TaskQueued {
			demand.queuedCount++
		}
		if task.CreateTime.Valid && !task.CreateTime.Time.Before(windowStart) {
			demand.arrivalCount++
		}
		if task.CreateTime.Valid && task.CreateTime.Time.After(demand.latestCreateTime) {
			demand.latestCreateTime = task.CreateTime.Time
			demand.latestTaskType = task.TaskType
		}
		if task.EstimatedNodeSeconds > 0 {
			demand.estimatedSecondsSum += task.EstimatedNodeSeconds
			demand.estimatedSecondsCnt++
		}
	}

	completedTasks, err := getCompletedTasksInWindow(ctx, db, windowStart)
	if err != nil {
		return nil, err
	}
	for i := range completedTasks {
		task := &completedTasks[i]
		executionSeconds := task.ScoreReadyTime.Time.Sub(task.StartTime.Time).Seconds()
		if executionSeconds <= 0 {
			continue
		}
		key, _, _, ok, err := modelDemandKeyOf(task)
		if err != nil {
			return nil, fmt.Errorf("task %d required model files: %w", task.ID, err)
		}
		if !ok {
			continue
		}
		demand, ok := demands[key]
		if !ok {
			continue
		}
		demand.executionSecondsSum += executionSeconds
		demand.executionSecondsCnt++
	}
	return demands, nil
}

func getDemandTasks(ctx context.Context, db *gorm.DB, windowStart time.Time) ([]models.InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var tasks []models.InferenceTask
	if err := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
		Select("id, status, task_type, required_model_file_hashes, required_model_file_source_hashes, create_time, estimated_node_seconds, min_v_ram, required_gpu, required_gpuv_ram").
		Where("status = ? OR create_time >= ?", models.TaskQueued, windowStart).
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func getCompletedTasksInWindow(ctx context.Context, db *gorm.DB, windowStart time.Time) ([]models.InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var tasks []models.InferenceTask
	if err := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
		Select("id, task_type, required_model_file_hashes, required_model_file_source_hashes, start_time, score_ready_time, min_v_ram, required_gpu, required_gpuv_ram").
		Where("score_ready_time >= ?", windowStart).
		Where("start_time IS NOT NULL").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// computeTargetNodeCount derives the target holding-node count of one model
// file set: ceil(arrival_rate * avg_execution_seconds * safety_factor) clamped
// to [minNodes, maxNodes]. When the window contains no completed task, the
// mean stored pricing estimate of the demanding tasks is used instead.
func computeTargetNodeCount(demand *modelDemand, windowSeconds, safetyFactor float64, minNodes, maxNodes int) int {
	arrivalRate := float64(demand.arrivalCount) / windowSeconds

	var avgExecutionSeconds float64
	if demand.executionSecondsCnt > 0 {
		avgExecutionSeconds = demand.executionSecondsSum / float64(demand.executionSecondsCnt)
	} else if demand.estimatedSecondsCnt > 0 {
		avgExecutionSeconds = demand.estimatedSecondsSum / float64(demand.estimatedSecondsCnt)
	}

	target := int(math.Ceil(arrivalRate * avgExecutionSeconds * safetyFactor))
	if target < minNodes {
		target = minNodes
	}
	if target > maxNodes {
		target = maxNodes
	}
	return target
}

func buildModelSelectionStates(selections []models.NodeModelDownloadSelection) (map[models.Hash32]*modelSelectionState, error) {
	states := make(map[models.Hash32]*modelSelectionState)
	for _, selection := range selections {
		setKey, err := models.Hash32FromBytes(selection.ModelFileHash)
		if err != nil {
			return nil, err
		}
		state, ok := states[setKey]
		if !ok {
			state = newModelSelectionState()
			states[setKey] = state
		}
		state.attemptCounts[selection.NodeAddress]++
		switch selection.Status {
		case models.NodeModelDownloadSelectionPending:
			state.pendingNodes[selection.NodeAddress] = struct{}{}
			state.nonExpired[selection.NodeAddress] = struct{}{}
		case models.NodeModelDownloadSelectionCompleted:
			state.nonExpired[selection.NodeAddress] = struct{}{}
		}
	}
	return states, nil
}

// cleanupSelectionsWithoutDemand deletes the selection records of model file
// sets that no longer have current demand in any demand group, ending their
// demand period.
func cleanupSelectionsWithoutDemand(ctx context.Context, db *gorm.DB, selectionStates map[models.Hash32]*modelSelectionState, demands map[modelDemandKey]*modelDemand) error {
	demandedSets := make(map[models.Hash32]struct{})
	for key := range demands {
		demandedSets[key.setKey] = struct{}{}
	}
	staleSets := make([]models.Hash32, 0)
	for setKey := range selectionStates {
		if _, ok := demandedSets[setKey]; !ok {
			staleSets = append(staleSets, setKey)
		}
	}
	return models.DeleteNodeModelDownloadSelectionsByModelFileHashes(ctx, db, staleSets)
}

// getModelHoldingNodes returns, per demand group, the addresses of the given
// Available or Busy nodes whose on-disk inventory covers the required model
// file set: every exact file and at least one file of every source.
func getModelHoldingNodes(ctx context.Context, db *gorm.DB, demands map[modelDemandKey]*modelDemand, nodes []models.Node) (map[modelDemandKey]map[string]struct{}, error) {
	addresses := make([]string, 0, len(nodes))
	for _, node := range nodes {
		addresses = append(addresses, node.Address)
	}
	holding := make(map[modelDemandKey]map[string]struct{}, len(demands))
	if len(addresses) == 0 {
		return holding, nil
	}
	rows, err := models.ListNodeModelFileRows(ctx, db, addresses...)
	if err != nil {
		return nil, err
	}
	inventories, err := nodeFileSetsByAddress(rows)
	if err != nil {
		return nil, err
	}
	for key, demand := range demands {
		covering := make(map[string]struct{})
		for address, inventory := range inventories {
			if modelFilesCovered(inventory.exact, inventory.source, demand.exact, demand.source) {
				covering[address] = struct{}{}
			}
		}
		holding[key] = covering
	}
	return holding, nil
}

func getDownloadCandidateNodes(ctx context.Context, db *gorm.DB) ([]models.Node, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodes []models.Node
	if err := db.WithContext(dbCtx).Model(&models.Node{}).
		Where("status IN ?", []models.NodeStatus{models.NodeStatusAvailable, models.NodeStatusBusy}).
		Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// selectDownloadTargetNodes selects up to limit download target nodes for one
// model file set. The primary pool contains candidate nodes that do not hold
// the set and have no selection record for it in the current demand period.
// Only when the primary pool is empty are nodes with expired-only records
// re-admitted, ordered so that every eligible node is attempted once before
// any node is attempted twice.
func selectDownloadTargetNodes(candidateNodes []models.Node, holding map[string]struct{}, state *modelSelectionState, now time.Time, limit int) []models.Node {
	primary := make([]models.Node, 0, len(candidateNodes))
	readmittable := make([]models.Node, 0)
	for _, node := range candidateNodes {
		if _, ok := holding[node.Address]; ok {
			continue
		}
		attempts := 0
		if state != nil {
			attempts = state.attemptCounts[node.Address]
		}
		if attempts == 0 {
			primary = append(primary, node)
			continue
		}
		if _, ok := state.nonExpired[node.Address]; !ok {
			readmittable = append(readmittable, node)
		}
	}

	if len(primary) > 0 {
		return sampleDownloadTargets(primary, now, limit)
	}

	// Group re-admitted nodes by ascending attempt count so every eligible
	// node is attempted once before any node is attempted twice.
	groups := make(map[int][]models.Node)
	attemptCounts := make([]int, 0)
	for _, node := range readmittable {
		attempts := state.attemptCounts[node.Address]
		if _, ok := groups[attempts]; !ok {
			attemptCounts = append(attemptCounts, attempts)
		}
		groups[attempts] = append(groups[attempts], node)
	}
	sort.Ints(attemptCounts)

	selected := make([]models.Node, 0, limit)
	for _, attempts := range attemptCounts {
		if len(selected) == limit {
			break
		}
		selected = append(selected, sampleDownloadTargets(groups[attempts], now, limit-len(selected))...)
	}
	return selected
}

// sampleDownloadTargets draws up to limit nodes by weighted sampling without
// replacement using the staking and QoS base weights. Zero-weight candidates
// are selected only after all positive-weight candidates are exhausted, in
// ascending node address order.
func sampleDownloadTargets(nodes []models.Node, now time.Time, limit int) []models.Node {
	if limit > len(nodes) {
		limit = len(nodes)
	}
	if limit <= 0 {
		return nil
	}

	positiveNodes := make([]models.Node, 0, len(nodes))
	positiveWeights := make([]float64, 0, len(nodes))
	zeroNodes := make([]models.Node, 0)
	for _, node := range nodes {
		weight := CalculateNodeSelectingProb(node, now).ProbWeight
		if weight > 0 {
			positiveNodes = append(positiveNodes, node)
			positiveWeights = append(positiveWeights, weight)
		} else {
			zeroNodes = append(zeroNodes, node)
		}
	}
	sort.Slice(zeroNodes, func(i, j int) bool {
		return zeroNodes[i].Address < zeroNodes[j].Address
	})

	selected := make([]models.Node, 0, limit)
	if len(positiveNodes) > 0 {
		weighted := sampleuv.NewWeighted(positiveWeights, nil)
		for len(selected) < limit {
			index, ok := weighted.Take()
			if !ok {
				break
			}
			selected = append(selected, positiveNodes[index])
		}
	}
	for _, node := range zeroNodes {
		if len(selected) == limit {
			break
		}
		selected = append(selected, node)
	}
	return selected
}

// loadDemandModelFiles returns the model file requirements of a demand group:
// exact entries with their revision from the catalog and source entries with
// an empty revision derived from the TaskArgs of a demanding task.
func loadDemandModelFiles(ctx context.Context, db *gorm.DB, demand *modelDemand) ([]models.ModelFileEntry, error) {
	exactEntries, err := LoadModelFileEntries(ctx, db, demand.exact)
	if err != nil {
		return nil, err
	}
	files := make([]models.ModelFileEntry, 0, len(demand.exact)+len(demand.source))
	for _, hash := range demand.exact {
		entry, ok := exactEntries[hash]
		if !ok {
			return nil, fmt.Errorf("model file %s is missing from the catalog", hash.Hex())
		}
		files = append(files, entry)
	}
	if len(demand.source) == 0 {
		return files, nil
	}

	var task models.InferenceTask
	if err := db.WithContext(ctx).Select("id, task_type, task_args").First(&task, demand.representativeTask).Error; err != nil {
		return nil, err
	}
	requirements, err := models.DeriveTaskModelRequirements(task.TaskArgs, task.TaskType)
	if err != nil {
		return nil, err
	}
	wanted := hashSet(demand.source)
	for _, entry := range requirements.Entries {
		if entry.Revision != "" {
			continue
		}
		sourceHash, err := entry.ModelFileSourceHash()
		if err != nil {
			return nil, err
		}
		if _, ok := wanted[sourceHash]; ok {
			files = append(files, entry)
			delete(wanted, sourceHash)
		}
	}
	if len(wanted) > 0 {
		return nil, fmt.Errorf("task %d does not provide %d required model file sources", task.ID, len(wanted))
	}
	return files, nil
}

// emitModelDownloadSelection inserts the selection record and the DownloadModel
// event in one database transaction.
func emitModelDownloadSelection(ctx context.Context, db *gorm.DB, demand *modelDemand, modelFiles []models.ModelFileEntry, nodeAddress string, sentAt time.Time, downloadTimeoutSeconds float64) error {
	deadline := sentAt.Add(time.Duration(downloadTimeoutSeconds * float64(time.Second)))
	selection := models.NewNodeModelDownloadSelection(demand.exact, demand.source, nodeAddress, demand.key.minVRAM, sentAt, deadline)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := models.CreateNodeModelDownloadSelection(ctx, tx, selection); err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.DownloadModelEvent{
			NodeAddress: nodeAddress,
			ModelFiles:  modelFiles,
			TaskType:    demand.latestTaskType,
		})
	}); err != nil {
		return err
	}
	metrics.ModelDownloadsDispatched.WithLabelValues(metrics.TaskTypeLabel(demand.latestTaskType), metrics.VramTierLabel(demand.key.minVRAM)).Inc()
	return nil
}
