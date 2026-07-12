package metrics

import (
	"context"
	"crynux_relay/models"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// Registry is a dedicated registry so the /metrics endpoint only exposes
// relay application metrics.
var Registry = prometheus.NewRegistry()

var (
	TasksCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_tasks_created_total",
		Help: "Total number of inference tasks created, by task type and creator address.",
	}, []string{"task_type", "creator"})

	TasksDispatched = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_tasks_dispatched_total",
		Help: "Total number of inference tasks dispatched to a node (status Started).",
	}, []string{"task_type"})

	TasksDelivered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "relay_tasks_delivered_total",
		Help: "Total number of inference tasks fetched by their selected node for the first time.",
	})

	TasksErrorReported = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "relay_tasks_error_reported_total",
		Help: "Total number of inference tasks whose node reported a task error.",
	})

	TasksTerminal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_tasks_terminal_total",
		Help: "Total number of inference tasks reaching a terminal status, by terminal status and task type.",
	}, []string{"status", "task_type"})

	TasksAborted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_tasks_aborted_total",
		Help: "Total number of aborted inference tasks, by abort reason and the task status before the abort.",
	}, []string{"reason", "status"})

	TaskQueueWaitSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_task_queue_wait_seconds",
		Help:    "Time spent by tasks in the queue between creation and dispatch.",
		Buckets: []float64{1, 2, 5, 10, 30, 60, 120, 300, 600, 1800},
	}, []string{"task_type", "vram_tier"})

	TaskExecutionSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_task_execution_seconds",
		Help:    "Time spent by tasks between dispatch and score ready.",
		Buckets: []float64{5, 10, 30, 60, 120, 300, 600, 1200, 1800, 3600},
	}, []string{"task_type", "vram_tier"})

	NodeSelectionCandidates = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_node_selection_candidates",
		Help:    "Size of the final candidate node pool observed during node selection for inference tasks.",
		Buckets: []float64{0, 1, 2, 5, 10, 20, 50, 100, 200},
	}, []string{"task_type", "vram_tier", "gpu"})

	NodeSelectionEmpty = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_node_selection_empty_total",
		Help: "Total number of node selection attempts that found no candidate node.",
	}, []string{"task_type", "vram_tier", "gpu"})

	NodeHealthPenalties = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "relay_node_health_penalties_total",
		Help: "Total number of health penalties applied to nodes after task timeouts.",
	})

	NodeEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_node_events_total",
		Help: "Total number of node lifecycle events, by event type.",
	}, []string{"event"})

	TaskQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "relay_task_queue_depth",
		Help: "Current number of tasks in queued status.",
	})

	Nodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_nodes",
		Help: "Current number of nodes, by node status.",
	}, []string{"status"})

	NodesFailing30m = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "relay_nodes_failing_30m",
		Help: "Number of distinct nodes with timeout-aborted tasks in the last 30 minutes.",
	})

	NodesAlive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "relay_nodes_alive",
		Help: "Number of nodes whose last task poll was within the last 2 minutes.",
	})
)

func init() {
	Registry.MustRegister(
		TasksCreated,
		TasksDispatched,
		TasksDelivered,
		TasksErrorReported,
		TasksTerminal,
		TasksAborted,
		TaskQueueWaitSeconds,
		TaskExecutionSeconds,
		NodeSelectionCandidates,
		NodeSelectionEmpty,
		NodeHealthPenalties,
		NodeEvents,
		TaskQueueDepth,
		Nodes,
		NodesFailing30m,
		NodesAlive,
	)
}

var vramTiers []uint64

// InitVramTiers stores the configured VRAM tier boundaries (in GB, ascending)
// used to map raw task VRAM requirements to low-cardinality tier labels.
func InitVramTiers(tiers []uint64) {
	vramTiers = append([]uint64(nil), tiers...)
	sort.Slice(vramTiers, func(i, j int) bool { return vramTiers[i] < vramTiers[j] })
}

// VramTierLabel maps a task's minimum VRAM requirement (in GB) to a tier label
// such as "0-8", "8-16" or "48+" based on the configured tier boundaries.
func VramTierLabel(minVram uint64) string {
	if len(vramTiers) == 0 {
		return "all"
	}
	prev := uint64(0)
	for _, boundary := range vramTiers {
		if minVram < boundary {
			return fmt.Sprintf("%d-%d", prev, boundary)
		}
		prev = boundary
	}
	return fmt.Sprintf("%d+", vramTiers[len(vramTiers)-1])
}

// GPULabel returns the exact required GPU name for GPU-pinned tasks, or "any".
func GPULabel(requiredGPU string) string {
	if requiredGPU == "" {
		return "any"
	}
	return requiredGPU
}

// TaskTypeLabel maps a task type to a stable metric label.
func TaskTypeLabel(taskType models.TaskType) string {
	switch taskType {
	case models.TaskTypeSD:
		return "sd"
	case models.TaskTypeLLM:
		return "llm"
	case models.TaskTypeSDFTLora:
		return "sd_ft_lora"
	default:
		return "unknown"
	}
}

// AbortReasonLabel maps a task abort reason to a stable metric label.
func AbortReasonLabel(reason models.TaskAbortReason) string {
	switch reason {
	case models.TaskAbortReasonNone:
		return "none"
	case models.TaskAbortTimeout:
		return "timeout"
	case models.TaskAbortModelDownloadFailed:
		return "model_download_failed"
	case models.TaskAbortIncorrectResult:
		return "incorrect_result"
	case models.TaskAbortTaskFeeTooLow:
		return "task_fee_too_low"
	default:
		return "unknown"
	}
}

// AbortStatusLabel maps the task status before an abort to a metric label.
// TaskStarted is split by whether the selected node had fetched the task:
// TaskStartedDelivered when delivered_time is set, TaskStartedUndelivered
// otherwise. All other statuses use their enum name.
func AbortStatusLabel(statusBeforeAbort models.TaskStatus, task *models.InferenceTask) string {
	if statusBeforeAbort == models.TaskStarted {
		if task.DeliveredTime.Valid {
			return "TaskStartedDelivered"
		}
		return "TaskStartedUndelivered"
	}
	return TaskStatusLabel(statusBeforeAbort)
}

// TaskStatusLabel maps a task status to its enum name used as a metric label.
func TaskStatusLabel(status models.TaskStatus) string {
	switch status {
	case models.TaskQueued:
		return "TaskQueued"
	case models.TaskStarted:
		return "TaskStarted"
	case models.TaskParametersUploaded:
		return "TaskParametersUploaded"
	case models.TaskErrorReported:
		return "TaskErrorReported"
	case models.TaskScoreReady:
		return "TaskScoreReady"
	case models.TaskValidated:
		return "TaskValidated"
	case models.TaskGroupValidated:
		return "TaskGroupValidated"
	case models.TaskEndInvalidated:
		return "TaskEndInvalidated"
	case models.TaskEndSuccess:
		return "TaskEndSuccess"
	case models.TaskEndAborted:
		return "TaskEndAborted"
	case models.TaskEndGroupRefund:
		return "TaskEndGroupRefund"
	case models.TaskEndGroupSuccess:
		return "TaskEndGroupSuccess"
	default:
		return "unknown"
	}
}

// NodeStatusLabel maps a node status to a stable metric label.
func NodeStatusLabel(status models.NodeStatus) string {
	switch status {
	case models.NodeStatusQuit:
		return "quit"
	case models.NodeStatusAvailable:
		return "available"
	case models.NodeStatusBusy:
		return "busy"
	case models.NodeStatusPendingPause:
		return "pending_pause"
	case models.NodeStatusPendingQuit:
		return "pending_quit"
	case models.NodeStatusPaused:
		return "paused"
	default:
		return "unknown"
	}
}

// StartMetricsServer serves the /metrics endpoint on a dedicated port so
// metrics are never exposed on the public API port.
func StartMetricsServer(ctx context.Context, port string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(Registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Errorf("Metrics: server shutdown error: %v", err)
		}
	}()

	log.Infof("Metrics: serving /metrics on port %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Errorf("Metrics: server error: %v", err)
	}
}
