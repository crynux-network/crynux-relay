package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"math"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/stat/sampleuv"
	"gorm.io/gorm"
)

// modelDemandKey identifies one distribution unit: a base model together
// with the VRAM demand and the LLM Darwin exclusion of the tasks requiring
// it. Tasks with different VRAM demands form separate demand groups because
// a node only counts toward coverage of the groups it can execute.
type modelDemandKey struct {
	modelID       string
	minVRAM       uint64
	excludeDarwin bool
}

// taskVRAMDemand is the VRAM amount a task requires from a node: the exact
// GPU VRAM when the task pins a GPU, otherwise the minimum VRAM bound.
func taskVRAMDemand(task *models.InferenceTask) uint64 {
	if len(task.RequiredGPU) > 0 {
		return task.RequiredGPUVRAM
	}
	return task.MinVRAM
}

func modelDemandKeyOf(task *models.InferenceTask, modelID string) modelDemandKey {
	return modelDemandKey{
		modelID:       modelID,
		minVRAM:       taskVRAMDemand(task),
		excludeDarwin: task.TaskType == models.TaskTypeLLM,
	}
}

// nodeSatisfiesDemand reports whether a node's VRAM covers the demand
// group's requirement and, for LLM demand, the node is not a Darwin node.
func nodeSatisfiesDemand(gpuName string, gpuVram uint64, key modelDemandKey) bool {
	if gpuVram < key.minVRAM {
		return false
	}
	if key.excludeDarwin && isDarwinGPUName(gpuName) {
		return false
	}
	return true
}

// modelDemand aggregates the demand facts of one demand group over the
// current demand window: queued tasks, window arrivals, measured execution
// durations and the pricing estimates used as the cold-start fallback.
type modelDemand struct {
	key                  modelDemandKey
	queuedCount          int
	arrivalCount         int
	latestCreateTime     time.Time
	latestTaskType       models.TaskType
	estimatedSecondsSum  float64
	estimatedSecondsCnt  int
	executionSecondsSum  float64
	executionSecondsCnt  int
}

// modelSelectionState is the per-model view over the persisted selection
// records used for capacity accounting and pool exclusion.
type modelSelectionState struct {
	// attemptCounts counts all selection records per node address.
	attemptCounts map[string]int
	// nonExpired marks nodes with a pending or completed record.
	nonExpired map[string]struct{}
	// pendingNodes marks nodes with a pending record.
	pendingNodes map[string]struct{}
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
	selectionStates := buildModelSelectionStates(selections)

	if err := cleanupSelectionsWithoutDemand(ctx, db, selectionStates, demands); err != nil {
		return err
	}

	if len(demands) == 0 {
		return nil
	}

	demandModelIDs := demandedModelIDs(demands)
	holdingNodes, err := getModelHoldingNodes(ctx, db, demandModelIDs)
	if err != nil {
		return err
	}

	candidateNodes, err := getDownloadCandidateNodes(ctx, db)
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
		modelID := demand.key.modelID
		state := selectionStates[modelID]
		target := computeTargetNodeCount(demand, cfg.DemandWindowSeconds, cfg.SafetyFactor, cfg.MinNodes, cfg.MaxNodes)

		// Coverage and pending capacity count only nodes that satisfy the
		// demand group's VRAM requirement; a copy on a node that cannot
		// execute the demanding tasks does not serve them.
		holding := holdingNodes[modelID]
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
		for _, node := range selected {
			if err := emitModelDownloadSelection(ctx, db, modelID, node.Address, demand.key.minVRAM, demand.latestTaskType, now, cfg.DownloadTimeoutSeconds); err != nil {
				log.Errorf("ModelDistribution: emit download selection for model %s on node %s error: %v", modelID, node.Address, err)
				continue
			}
			// Record the new selection in the in-memory state so later
			// demand groups of the same model see it as pending capacity
			// and do not select the node again in this round.
			if state == nil {
				state = &modelSelectionState{
					attemptCounts: make(map[string]int),
					nonExpired:    make(map[string]struct{}),
					pendingNodes:  make(map[string]struct{}),
				}
				selectionStates[modelID] = state
			}
			state.attemptCounts[node.Address]++
			state.nonExpired[node.Address] = struct{}{}
			state.pendingNodes[node.Address] = struct{}{}
		}
	}
	return nil
}

func demandedModelIDs(demands map[modelDemandKey]*modelDemand) []string {
	modelIDSet := make(map[string]struct{})
	modelIDs := make([]string, 0, len(demands))
	for key := range demands {
		if _, ok := modelIDSet[key.modelID]; ok {
			continue
		}
		modelIDSet[key.modelID] = struct{}{}
		modelIDs = append(modelIDs, key.modelID)
	}
	return modelIDs
}

// applySelectionStatusTransitions moves pending selections to completed when
// the node's authoritative on-disk model set contains the model, and to
// expired when the deadline has passed without completion.
func applySelectionStatusTransitions(ctx context.Context, db *gorm.DB, now time.Time) error {
	selections, err := models.GetAllNodeModelDownloadSelections(ctx, db)
	if err != nil {
		return err
	}

	pendingModelIDs := make(map[string]struct{})
	for _, selection := range selections {
		if selection.Status == models.NodeModelDownloadSelectionPending {
			pendingModelIDs[selection.ModelID] = struct{}{}
		}
	}
	completedIDSet := make(map[uint]struct{})
	if len(pendingModelIDs) > 0 {
		modelIDs := make([]string, 0, len(pendingModelIDs))
		for modelID := range pendingModelIDs {
			modelIDs = append(modelIDs, modelID)
		}
		onDisk, err := getNodeModelPairs(ctx, db, modelIDs)
		if err != nil {
			return err
		}
		completedIDs := make([]uint, 0)
		for _, selection := range selections {
			if selection.Status != models.NodeModelDownloadSelectionPending {
				continue
			}
			if _, ok := onDisk[nodeModelPair{modelID: selection.ModelID, nodeAddress: selection.NodeAddress}]; ok {
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

type nodeModelPair struct {
	modelID     string
	nodeAddress string
}

func getNodeModelPairs(ctx context.Context, db *gorm.DB, modelIDs []string) (map[nodeModelPair]struct{}, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var rows []models.NodeModel
	if err := db.WithContext(dbCtx).Model(&models.NodeModel{}).
		Select("model_id, node_address").
		Where("model_id IN ?", modelIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	pairs := make(map[nodeModelPair]struct{}, len(rows))
	for _, row := range rows {
		pairs[nodeModelPair{modelID: row.ModelID, nodeAddress: row.NodeAddress}] = struct{}{}
	}
	return pairs, nil
}

// collectModelDemands measures demand per demand group (base model plus task
// VRAM demand) from tasks that are currently queued or were created within
// the demand window, plus execution durations of tasks completed within the
// window.
func collectModelDemands(ctx context.Context, db *gorm.DB, windowStart time.Time) (map[modelDemandKey]*modelDemand, error) {
	demands := make(map[modelDemandKey]*modelDemand)

	demandTasks, err := getDemandTasks(ctx, db, windowStart)
	if err != nil {
		return nil, err
	}
	for i := range demandTasks {
		task := &demandTasks[i]
		for _, modelID := range models.BaseModelIDs(task.ModelIDs) {
			key := modelDemandKeyOf(task, modelID)
			demand, ok := demands[key]
			if !ok {
				demand = &modelDemand{key: key}
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
		for _, modelID := range models.BaseModelIDs(task.ModelIDs) {
			demand, ok := demands[modelDemandKeyOf(task, modelID)]
			if !ok {
				continue
			}
			demand.executionSecondsSum += executionSeconds
			demand.executionSecondsCnt++
		}
	}
	return demands, nil
}

func getDemandTasks(ctx context.Context, db *gorm.DB, windowStart time.Time) ([]models.InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var tasks []models.InferenceTask
	if err := db.WithContext(dbCtx).Model(&models.InferenceTask{}).
		Select("id, status, task_type, model_ids, create_time, estimated_node_seconds, min_v_ram, required_gpu, required_gpuv_ram").
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
		Select("id, task_type, model_ids, start_time, score_ready_time, min_v_ram, required_gpu, required_gpuv_ram").
		Where("score_ready_time >= ?", windowStart).
		Where("start_time IS NOT NULL").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// computeTargetNodeCount derives the target holding-node count of one base
// model: ceil(arrival_rate * avg_execution_seconds * safety_factor) clamped
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

func buildModelSelectionStates(selections []models.NodeModelDownloadSelection) map[string]*modelSelectionState {
	states := make(map[string]*modelSelectionState)
	for _, selection := range selections {
		state, ok := states[selection.ModelID]
		if !ok {
			state = &modelSelectionState{
				attemptCounts: make(map[string]int),
				nonExpired:    make(map[string]struct{}),
				pendingNodes:  make(map[string]struct{}),
			}
			states[selection.ModelID] = state
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
	return states
}

// cleanupSelectionsWithoutDemand deletes the selection records of models that
// no longer have current demand in any demand group, ending their demand
// period.
func cleanupSelectionsWithoutDemand(ctx context.Context, db *gorm.DB, selectionStates map[string]*modelSelectionState, demands map[modelDemandKey]*modelDemand) error {
	demandedIDs := make(map[string]struct{})
	for key := range demands {
		demandedIDs[key.modelID] = struct{}{}
	}
	staleModelIDs := make([]string, 0)
	for modelID := range selectionStates {
		if _, ok := demandedIDs[modelID]; !ok {
			staleModelIDs = append(staleModelIDs, modelID)
		}
	}
	return models.DeleteNodeModelDownloadSelectionsByModelIDs(ctx, db, staleModelIDs)
}

// getModelHoldingNodes returns, per base model, the addresses of currently
// joined Available or Busy nodes whose on-disk model set contains the model.
func getModelHoldingNodes(ctx context.Context, db *gorm.DB, modelIDs []string) (map[string]map[string]struct{}, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var rows []models.NodeModel
	if err := db.WithContext(dbCtx).Model(&models.NodeModel{}).
		Select("node_models.model_id, node_models.node_address").
		Joins("INNER JOIN nodes ON nodes.address = node_models.node_address AND nodes.deleted_at IS NULL").
		Where("node_models.model_id IN ?", modelIDs).
		Where("nodes.status IN ?", []models.NodeStatus{models.NodeStatusAvailable, models.NodeStatusBusy}).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	holding := make(map[string]map[string]struct{})
	for _, row := range rows {
		nodes, ok := holding[row.ModelID]
		if !ok {
			nodes = make(map[string]struct{})
			holding[row.ModelID] = nodes
		}
		nodes[row.NodeAddress] = struct{}{}
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
// base model. The primary pool contains candidate nodes that do not hold the
// model and have no selection record for it in the current demand period.
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

// emitModelDownloadSelection inserts the selection record and the DownloadModel
// event in one database transaction.
func emitModelDownloadSelection(ctx context.Context, db *gorm.DB, modelID, nodeAddress string, minVRAM uint64, taskType models.TaskType, sentAt time.Time, downloadTimeoutSeconds float64) error {
	deadline := sentAt.Add(time.Duration(downloadTimeoutSeconds * float64(time.Second)))
	selection := models.NewNodeModelDownloadSelection(modelID, nodeAddress, minVRAM, sentAt, deadline)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := models.CreateNodeModelDownloadSelection(ctx, tx, selection); err != nil {
			return err
		}
		return emitEvent(ctx, tx, &models.DownloadModelEvent{
			NodeAddress: nodeAddress,
			ModelID:     selection.ModelID,
			TaskType:    taskType,
		})
	}); err != nil {
		return err
	}
	metrics.ModelDownloadsDispatched.WithLabelValues(metrics.TaskTypeLabel(taskType), metrics.VramTierLabel(minVRAM)).Inc()
	return nil
}
