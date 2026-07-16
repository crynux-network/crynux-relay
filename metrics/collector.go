package metrics

import (
	"context"
	"crynux_relay/models"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// modelNodesTopN caps the number of models exported by the relay_model_nodes
// gauge to bound the hf_model_id label cardinality.
const modelNodesTopN = 50

var gaugeNodeStatuses = []models.NodeStatus{
	models.NodeStatusQuit,
	models.NodeStatusAvailable,
	models.NodeStatusBusy,
	models.NodeStatusPendingPause,
	models.NodeStatusPendingQuit,
	models.NodeStatusPaused,
}

// StartGaugeCollector periodically refreshes DB-backed gauges: task queue
// depth, node counts by status, distinct failing nodes in the last 30 minutes
// and alive nodes seen within the last 2 minutes.
func StartGaugeCollector(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	collectGauges(ctx, db)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectGauges(ctx, db)
		}
	}
}

func collectGauges(ctx context.Context, db *gorm.DB) {
	now := time.Now()

	if queueDepth, err := models.GetQueuedTaskCount(ctx, db); err != nil {
		log.Errorf("Metrics: failed to get queued task count: %v", err)
	} else {
		TaskQueueDepth.Set(float64(queueDepth))
	}

	if nodeCounts, err := models.GetNodeCountsByStatus(ctx, db); err != nil {
		log.Errorf("Metrics: failed to get node counts by status: %v", err)
	} else {
		for _, status := range gaugeNodeStatuses {
			Nodes.WithLabelValues(NodeStatusLabel(status)).Set(float64(nodeCounts[status]))
		}
	}

	if failingNodes, err := models.GetTimeoutAbortedNodeCount(ctx, db, now.Add(-30*time.Minute)); err != nil {
		log.Errorf("Metrics: failed to get failing node count: %v", err)
	} else {
		NodesFailing30m.Set(float64(failingNodes))
	}

	if aliveNodes, err := models.GetAliveNodeCount(ctx, db, now.Add(-2*time.Minute)); err != nil {
		log.Errorf("Metrics: failed to get alive node count: %v", err)
	} else {
		NodesAlive.Set(float64(aliveNodes))
	}

	if modelCounts, err := models.CountNodesByHFModelID(ctx, db); err != nil {
		log.Errorf("Metrics: failed to get node counts by model: %v", err)
	} else {
		SetModelNodes(topModelNodeCounts(modelCounts, modelNodesTopN))
	}
}

// topModelNodeCounts ranks the per-model node counts by on-disk node count
// descending, breaking ties by in-memory node count descending and then by
// model ID ascending, and returns the top limit entries.
func topModelNodeCounts(counts map[string]models.HFModelNodeCount, limit int) []ModelNodeCount {
	entries := make([]ModelNodeCount, 0, len(counts))
	for hfModelID, count := range counts {
		entries = append(entries, ModelNodeCount{
			HFModelID: hfModelID,
			OnDisk:    count.OnDisk,
			InMemory:  count.InMemory,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].OnDisk != entries[j].OnDisk {
			return entries[i].OnDisk > entries[j].OnDisk
		}
		if entries[i].InMemory != entries[j].InMemory {
			return entries[i].InMemory > entries[j].InMemory
		}
		return entries[i].HFModelID < entries[j].HFModelID
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}
