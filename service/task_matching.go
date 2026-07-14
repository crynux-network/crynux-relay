package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/stat/sampleuv"
)

// matchingCandidateSet is the computed candidate set for one requirement
// signature: the index entries passing the hard filters and the model
// locality restriction, with their sampling weights.
type matchingCandidateSet struct {
	entries []*NodeIndexEntry
	scores  []float64
	probs   []NodeSelectingProb
}

// matchedPair is one task-node reservation produced by a matching round.
type matchedPair struct {
	task                   *models.InferenceTask
	nodeAddress            string
	candidatePool          []TaskTraceNodeSelectionCandidate
	candidatePoolTotal     int
	candidatePoolTruncated bool
	matchedAt              time.Time
}

// requirementSignature identifies the tuple that determines a task's
// candidate set, so tasks sharing it reuse one computed set within a round.
func requirementSignature(task *models.InferenceTask) string {
	return fmt.Sprintf("%d|%d|%s|%d|%s|%s",
		task.TaskType, task.MinVRAM, task.RequiredGPU, task.RequiredGPUVRAM, task.TaskVersion,
		strings.Join(task.ModelIDs, ";"))
}

func entryMatchesTaskVersion(entry *NodeIndexEntry, taskVersionNumbers [3]uint64) bool {
	return entry.MajorVersion == taskVersionNumbers[0] &&
		(entry.MinorVersion > taskVersionNumbers[1] ||
			(entry.MinorVersion == taskVersionNumbers[1] && entry.PatchVersion >= taskVersionNumbers[2]))
}

func isDarwinEntry(entry *NodeIndexEntry) bool {
	names := strings.SplitN(entry.GPUName, "+", 2)
	if len(names) != 2 {
		return true
	}
	return strings.TrimSpace(names[1]) == "Darwin"
}

// filterIndexEntriesForTask applies the hard filters over the round's node
// view: node availability, GPU or VRAM requirement, version compatibility and
// the LLM Darwin exclusion.
func filterIndexEntriesForTask(task *models.InferenceTask, entries []*NodeIndexEntry) []*NodeIndexEntry {
	taskVersionNumbers := task.VersionNumbers()
	filtered := make([]*NodeIndexEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Status != models.NodeStatusAvailable || entry.HasCurrentTask {
			continue
		}
		if !entryMatchesTaskVersion(entry, taskVersionNumbers) {
			continue
		}
		if len(task.RequiredGPU) > 0 {
			if entry.GPUName != task.RequiredGPU || entry.GPUVram != task.RequiredGPUVRAM {
				continue
			}
		} else {
			if entry.GPUVram < task.MinVRAM {
				continue
			}
			if task.TaskType == models.TaskTypeLLM && isDarwinEntry(entry) {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func filterIndexEntriesByNodeNamePolicy(ctx context.Context, entries []*NodeIndexEntry) ([]*NodeIndexEntry, error) {
	cfg := config.GetConfig()
	minimumNodeNameNumber := cfg.Task.MinimumNodeNameNumber
	nodeNameWhitelistEnabled := cfg.Task.NodeNameWhitelistEnabled
	if minimumNodeNameNumber == 0 && !nodeNameWhitelistEnabled {
		return entries, nil
	}

	filtered := make([]*NodeIndexEntry, 0, len(entries))
	for _, entry := range entries {
		nodeVersion := BuildNodeVersion(entry.MajorVersion, entry.MinorVersion, entry.PatchVersion)
		if nodeNameWhitelistEnabled {
			allowed, err := IsNodeNameWhitelisted(ctx, config.GetDB(), entry.GPUName, entry.GPUVram, nodeVersion)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
		}
		if minimumNodeNameNumber > 0 {
			count, err := GetNodeNameActiveCount(ctx, config.GetDB(), entry.GPUName, entry.GPUVram, nodeVersion)
			if err != nil {
				return nil, err
			}
			if count < minimumNodeNameNumber {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
}

func matchEntryModels(modelIDSet map[string]struct{}, taskModelIDs []string) int {
	cnt := 0
	for _, taskModelID := range taskModelIDs {
		if _, ok := modelIDSet[taskModelID]; ok {
			cnt += 1
		}
	}
	return cnt
}

// buildMatchingCandidateSet computes the candidate set for one requirement
// signature: hard filters, node name policy, sampling weights and the model
// locality restriction with its weight boost.
func buildMatchingCandidateSet(ctx context.Context, task *models.InferenceTask, entries []*NodeIndexEntry) (*matchingCandidateSet, error) {
	filtered := filterIndexEntriesForTask(task, entries)
	filtered, err := filterIndexEntriesByNodeNamePolicy(ctx, filtered)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	scores := make([]float64, len(filtered))
	probs := make([]NodeSelectingProb, len(filtered))
	for i, entry := range filtered {
		prob := CalculateNodeSelectingProb(entry.scoreNode(), now)
		scores[i] = prob.ProbWeight
		probs[i] = prob
	}

	// Boost nodes that have task models locally. Two cache layers are weighted
	// independently: disk presence (0.7) avoids expensive network downloads,
	// memory presence (0.3) avoids disk-to-GPU loading. Since in-use models
	// are a subset of local models, in-memory always gets a strictly higher
	// boost than on-disk-only. When any node has a task model on disk, the
	// candidate set is restricted to those nodes.
	changedEntries := make([]*NodeIndexEntry, 0)
	changedScores := make([]float64, 0)
	changedProbs := make([]NodeSelectingProb, 0)
	for i, entry := range filtered {
		cnt := matchEntryModels(entry.OnDiskModelIDs, task.ModelIDs)
		if cnt > 0 {
			inUseCnt := matchEntryModels(entry.InUseModelIDs, task.ModelIDs)
			total := float64(len(task.ModelIDs))
			changedEntries = append(changedEntries, entry)
			changedScores = append(changedScores, scores[i]*(1+0.7*float64(cnt)/total+0.3*float64(inUseCnt)/total))
			changedProbs = append(changedProbs, probs[i])
		}
	}
	if len(changedEntries) > 0 {
		filtered = changedEntries
		scores = changedScores
		probs = changedProbs
	}

	return &matchingCandidateSet{
		entries: filtered,
		scores:  scores,
		probs:   probs,
	}, nil
}

// sampleCandidateIndex draws one candidate index by weighted random sampling.
// It returns -1 when no candidate can be drawn.
func sampleCandidateIndex(scores []float64) int {
	if len(scores) == 0 {
		return -1
	}
	w := sampleuv.NewWeighted(scores, nil)
	if idx, ok := w.Take(); ok {
		return idx
	}
	return 0
}

func buildEntryTraceCandidatePool(entries []*NodeIndexEntry, probs []NodeSelectingProb, scores []float64) ([]TaskTraceNodeSelectionCandidate, int, bool) {
	totalCount := len(entries)
	limit := totalCount
	truncated := false
	if limit > taskTraceCandidatePoolLimit {
		limit = taskTraceCandidatePoolLimit
		truncated = true
	}

	candidatePool := make([]TaskTraceNodeSelectionCandidate, 0, limit)
	for i := 0; i < limit; i++ {
		candidatePool = append(candidatePool, TaskTraceNodeSelectionCandidate{
			Address:      entries[i].Address,
			CardName:     entries[i].GPUName,
			StakingScore: probs[i].StakingScore,
			QOSScore:     probs[i].QOSScore,
			ProbWeight:   scores[i],
		})
	}
	return candidatePool, totalCount, truncated
}

// matchTaskFromCandidateSet selects one node for the task by weighted random
// sampling over the shared candidate set minus nodes already reserved in this
// round. It returns nil when the task's candidate set is empty, recording the
// task in emptyPoolCounts under its selection label tuple.
func matchTaskFromCandidateSet(task *models.InferenceTask, set *matchingCandidateSet, reserved map[string]struct{}, traceSelection bool, emptyPoolCounts map[metrics.SelectionLabels]int) *matchedPair {
	availableEntries := make([]*NodeIndexEntry, 0, len(set.entries))
	availableScores := make([]float64, 0, len(set.entries))
	availableProbs := make([]NodeSelectingProb, 0, len(set.entries))
	for i, entry := range set.entries {
		if _, ok := reserved[entry.Address]; ok {
			continue
		}
		availableEntries = append(availableEntries, entry)
		availableScores = append(availableScores, set.scores[i])
		availableProbs = append(availableProbs, set.probs[i])
	}

	taskTypeLabel := metrics.TaskTypeLabel(task.TaskType)
	vramTierLabel := metrics.VramTierLabel(task.MinVRAM)
	gpuLabel := metrics.GPULabel(task.RequiredGPU)
	metrics.NodeSelectionCandidates.WithLabelValues(taskTypeLabel, vramTierLabel, gpuLabel).Observe(float64(len(availableEntries)))
	if len(availableEntries) == 0 {
		if emptyPoolCounts != nil {
			emptyPoolCounts[metrics.SelectionLabels{TaskType: taskTypeLabel, VramTier: vramTierLabel, GPU: gpuLabel}]++
		}
		return nil
	}

	idx := sampleCandidateIndex(availableScores)
	if idx < 0 {
		return nil
	}

	pair := &matchedPair{
		task:        task,
		nodeAddress: availableEntries[idx].Address,
		matchedAt:   time.Now(),
	}
	if traceSelection {
		pair.candidatePool, pair.candidatePoolTotal, pair.candidatePoolTruncated = buildEntryTraceCandidatePool(availableEntries, availableProbs, availableScores)
	}
	return pair
}

func fetchQueuedTaskPage(ctx context.Context, lastPriority *big.Int, lastID uint, limit int) ([]*models.InferenceTask, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := config.GetDB().WithContext(dbCtx).Model(&models.InferenceTask{}).
		Where("status = ?", models.TaskQueued)
	if lastPriority != nil {
		query = query.Where(
			"priority < ? OR (priority = ? AND id > ?)",
			models.BigInt{Int: *lastPriority}, models.BigInt{Int: *lastPriority}, lastID,
		)
	}

	tasks := make([]*models.InferenceTask, 0, limit)
	if err := query.Order("priority DESC, id ASC").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func hasUnreservedEligibleNode(entries []*NodeIndexEntry, reserved map[string]struct{}) bool {
	for _, entry := range entries {
		if entry.Status != models.NodeStatusAvailable || entry.HasCurrentTask {
			continue
		}
		if _, ok := reserved[entry.Address]; ok {
			continue
		}
		return true
	}
	return false
}

// runMatchingRound executes one scheduler iteration: it pairs queued tasks in
// priority order with nodes from the current index view, then runs the task
// start transactions for all matched pairs. It returns the number of matched
// pairs.
func runMatchingRound(ctx context.Context) (int, error) {
	batchSize := config.GetConfig().TaskMatching.BatchSize
	entries := SnapshotNodeIndex()

	reserved := make(map[string]struct{})
	candidateSets := make(map[string]*matchingCandidateSet)
	pairs := make([]*matchedPair, 0)
	emptyPoolCounts := make(map[metrics.SelectionLabels]int)
	traceSelection := GetTaskTraceStore().Enabled()

	var lastPriority *big.Int
	var lastID uint
	for {
		tasks, err := fetchQueuedTaskPage(ctx, lastPriority, lastID, batchSize)
		if err != nil {
			return len(pairs), err
		}
		if len(tasks) == 0 {
			break
		}

		now := time.Now()
		for _, task := range tasks {
			if isQueuedTaskTimedOut(task, now) {
				continue
			}
			signature := requirementSignature(task)
			set, ok := candidateSets[signature]
			if !ok {
				set, err = buildMatchingCandidateSet(ctx, task, entries)
				if err != nil {
					return len(pairs), err
				}
				candidateSets[signature] = set
			}
			pair := matchTaskFromCandidateSet(task, set, reserved, traceSelection, emptyPoolCounts)
			logTaskAssignmentEvent(ctx, task, len(set.entries))
			if pair == nil {
				continue
			}
			reserved[pair.nodeAddress] = struct{}{}
			pairs = append(pairs, pair)
		}

		if len(tasks) < batchSize {
			break
		}
		if !hasUnreservedEligibleNode(entries, reserved) {
			break
		}
		lastTask := tasks[len(tasks)-1]
		lastPriority = new(big.Int).Set(&lastTask.Priority.Int)
		lastID = lastTask.ID
	}

	metrics.SetNodeSelectionEmptyPoolTasks(emptyPoolCounts)
	startMatchedPairs(ctx, pairs)
	return len(pairs), nil
}

// startMatchedPairs runs the task start transaction for every matched pair.
// Executions run concurrently across pairs. Each start runs under the node's
// index lock, and the node's index entry is refreshed from the database
// whether the start succeeds or fails, so a failed pair leaves the task
// queued and the node resynced before reuse.
func startMatchedPairs(ctx context.Context, pairs []*matchedPair) {
	limiter := make(chan struct{}, 100)
	var wg sync.WaitGroup
	for _, pair := range pairs {
		wg.Add(1)
		go func(pair *matchedPair) {
			defer wg.Done()
			limiter <- struct{}{}
			defer func() { <-limiter }()
			startMatchedPair(ctx, pair)
		}(pair)
	}
	wg.Wait()
}

func startMatchedPair(ctx context.Context, pair *matchedPair) {
	task := pair.task
	err := ExecuteNodeStateUpdate(ctx, config.GetDB(), []string{pair.nodeAddress}, func() error {
		node, err := models.GetNodeWithModelsByAddress(ctx, config.GetDB(), pair.nodeAddress)
		if err != nil {
			return err
		}
		return SetTaskStatusStarted(ctx, config.GetDB(), task, node)
	})
	if err == nil {
		GetTaskTraceStore().RecordNodeSelected(
			task.TaskIDCommitment,
			pair.nodeAddress,
			pair.matchedAt,
			pair.candidatePool,
			pair.candidatePoolTotal,
			pair.candidatePoolTruncated,
		)
		log.Debugf("TaskMatching: task %s started on node %s", task.TaskIDCommitment, pair.nodeAddress)
		return
	}
	if errors.Is(err, errWrongTaskStatus) || errors.Is(err, models.ErrTaskStatusChanged) {
		log.Debugf("TaskMatching: task %s start skipped because task status changed", task.TaskIDCommitment)
	} else if errors.Is(err, models.ErrNodeStatusChanged) {
		log.Debugf("TaskMatching: task %s start failed because node %s status changed", task.TaskIDCommitment, pair.nodeAddress)
	} else {
		log.Errorf("TaskMatching: start task %s on node %s error: %v", task.TaskIDCommitment, pair.nodeAddress, err)
	}
}

// runMatchingScheduler runs the single task matching scheduler loop: the next
// round starts immediately when the current round matched at least one task,
// and otherwise after the configured tick interval.
func runMatchingScheduler(ctx context.Context) {
	tickInterval := time.Duration(config.GetConfig().TaskMatching.TickIntervalSeconds * float64(time.Second))
	timer := time.NewTimer(tickInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		matched, err := runMatchingRound(ctx)
		if err != nil {
			log.Errorf("TaskMatching: matching round error: %v", err)
		}
		if matched > 0 {
			continue
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(tickInterval)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
