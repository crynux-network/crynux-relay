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
		Help: "Total number of inference tasks created, by task type, creator address and VRAM tier.",
	}, []string{"task_type", "creator", "vram_tier"})

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
		Help: "Total number of inference tasks reaching a terminal status, by terminal status, task type and VRAM tier.",
	}, []string{"status", "task_type", "vram_tier"})

	TasksAborted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_tasks_aborted_total",
		Help: "Total number of aborted inference tasks, by abort reason, the task status before the abort, task type and VRAM tier.",
	}, []string{"reason", "status", "task_type", "vram_tier"})

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

	NodeSelectionEmptyPoolTasks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_node_selection_empty_pool_tasks",
		Help: "Number of queued tasks whose candidate node pool was empty in the latest matching round.",
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
		Help: "Number of distinct selected nodes with node-attributed execution or result-upload timeout aborts in the last 30 minutes.",
	})

	NodesAlive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "relay_nodes_alive",
		Help: "Number of nodes whose last task poll was within the last 2 minutes.",
	})

	TaskPricingSDOverheadSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_task_pricing_sd_overhead_seconds",
		Help: "Aggregated stable diffusion execution overhead seconds.",
	}, []string{"model_name", "model_variant", "requested_dtype", "quantize_bits", "vram_demand"})

	TaskPricingSecondsPerSDPixelStep = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_task_pricing_seconds_per_sd_pixel_step",
		Help: "Aggregated stable diffusion execution seconds per pixel-step.",
	}, []string{"model_name", "model_variant", "requested_dtype", "quantize_bits", "vram_demand"})

	TaskPricingLLMCoefficient = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_task_pricing_llm_coefficient",
		Help: "Aggregated LLM execution coefficient in its base unit.",
	}, []string{"coefficient", "model_name", "model_variant", "requested_dtype", "quantize_bits", "vram_demand"})

	GPUExecutionSDOverheadSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_gpu_execution_sd_overhead_seconds",
		Help: "Stable diffusion execution overhead seconds for an exact GPU variant.",
	}, []string{"task_type", "gpu_name", "gpu_vram", "model_name", "model_variant", "execution_dtype", "quantize_bits", "min_vram_requirement", "max_vram_requirement"})

	GPUExecutionSecondsPerSDPixelStep = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_gpu_execution_seconds_per_sd_pixel_step",
		Help: "Stable diffusion execution seconds per pixel-step for an exact GPU variant.",
	}, []string{"task_type", "gpu_name", "gpu_vram", "model_name", "model_variant", "execution_dtype", "quantize_bits", "min_vram_requirement", "max_vram_requirement"})

	GPUExecutionLLMCoefficient = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_gpu_execution_llm_coefficient",
		Help: "LLM execution coefficient for an exact GPU variant in its base unit.",
	}, []string{"coefficient", "task_type", "gpu_name", "gpu_vram", "model_name", "model_variant", "execution_dtype", "quantize_bits", "min_vram_requirement", "max_vram_requirement"})

	GPUExecutionCalibrationSamples = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_gpu_execution_calibration_samples",
		Help: "Cumulative valid calibration samples for an exact GPU variant and task type.",
	}, []string{"task_type", "gpu_name", "gpu_vram", "model_name", "model_variant", "execution_dtype", "quantize_bits", "min_vram_requirement", "max_vram_requirement"})

	TaskPriority = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_task_priority",
		Help:    "Distribution of computed task queue priority values at task creation, by task type.",
		Buckets: prometheus.ExponentialBuckets(1e9, 10, 12),
	}, []string{"task_type", "vram_demand"})

	ModelDownloadsDispatched = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_model_downloads_dispatched_total",
		Help: "Total number of DownloadModel events dispatched to nodes by the model distribution controller, by task type and demand VRAM tier.",
	}, []string{"task_type", "vram_tier"})

	ModelDownloadsCompleted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_model_downloads_completed_total",
		Help: "Total number of model download selections completed by the node reporting the model on disk, by demand VRAM tier.",
	}, []string{"vram_tier"})

	ModelDownloadsExpired = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_model_downloads_expired_total",
		Help: "Total number of model download selections expired at the download deadline without completion, by demand VRAM tier.",
	}, []string{"vram_tier"})

	ModelNodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "relay_model_nodes",
		Help: "Number of distinct nodes holding a huggingface base model, by model and holding state (on_disk or in_memory). Limited to the top models by on-disk node count.",
	}, []string{"hf_model_id", "state"})

	TaskBatchRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_task_batch_requests_total",
		Help: "Total task batch requests by endpoint and whole-request result.",
	}, []string{"endpoint", "result"})

	TaskBatchItems = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_task_batch_items_total",
		Help: "Total task batch items by endpoint and item outcome.",
	}, []string{"endpoint", "outcome"})

	TaskBatchDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_task_batch_duration_seconds",
		Help:    "Task batch request duration by endpoint.",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint"})

	TaskBatchResponseBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_task_batch_response_bytes_total",
		Help: "Total encoded task batch response bytes by endpoint.",
	}, []string{"endpoint"})

	TaskResultDownloadBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "relay_task_result_download_bytes_total",
		Help: "Total whole-task result bytes sent by task type.",
	}, []string{"task_type"})
)

// TaskExecutionTimeoutSeconds is created by InitTaskExecutionTimeoutBuckets from
// configured bucket upper bounds. It is nil until that init runs.
var TaskExecutionTimeoutSeconds *prometheus.HistogramVec

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
		NodeSelectionEmptyPoolTasks,
		NodeHealthPenalties,
		NodeEvents,
		TaskQueueDepth,
		Nodes,
		NodesFailing30m,
		NodesAlive,
		TaskPricingSDOverheadSeconds,
		TaskPricingSecondsPerSDPixelStep,
		TaskPricingLLMCoefficient,
		GPUExecutionSDOverheadSeconds,
		GPUExecutionSecondsPerSDPixelStep,
		GPUExecutionLLMCoefficient,
		GPUExecutionCalibrationSamples,
		TaskPriority,
		ModelDownloadsDispatched,
		ModelDownloadsCompleted,
		ModelDownloadsExpired,
		ModelNodes,
		TaskBatchRequests,
		TaskBatchItems,
		TaskBatchDurationSeconds,
		TaskBatchResponseBytes,
		TaskResultDownloadBytes,
	)
}

func ResetTaskPricingCalibrationMetrics() {
	TaskPricingSDOverheadSeconds.Reset()
	TaskPricingSecondsPerSDPixelStep.Reset()
	TaskPricingLLMCoefficient.Reset()
	GPUExecutionSDOverheadSeconds.Reset()
	GPUExecutionSecondsPerSDPixelStep.Reset()
	GPUExecutionLLMCoefficient.Reset()
	GPUExecutionCalibrationSamples.Reset()
}

func SetTaskPricingCalibration(taskType, modelName, modelVariant, requestedDType string, quantizeBits, vramDemand uint64, sdOverhead, sdRate float64, llm [6]float64) {
	vram := fmt.Sprint(vramDemand)
	labels := []string{modelName, modelVariant, requestedDType, fmt.Sprint(quantizeBits), vram}
	if taskType == "sd" {
		TaskPricingSDOverheadSeconds.WithLabelValues(labels...).Set(sdOverhead)
		TaskPricingSecondsPerSDPixelStep.WithLabelValues(labels...).Set(sdRate)
		return
	}
	if taskType == "llm" {
		for i, name := range []string{"constant", "text_input", "output", "model_switch", "image_count", "image_megapixel"} {
			TaskPricingLLMCoefficient.WithLabelValues(append([]string{name}, labels...)...).Set(llm[i])
		}
	}
}

func SetGPUExecutionCalibration(taskType, gpuName string, gpuVram uint64, modelName, modelVariant, executionDType string, quantizeBits, minVRAM, maxVRAM uint64, sdOverhead, sdRate float64, llm [6]float64, samples uint64) {
	labels := []string{taskType, gpuName, fmt.Sprint(gpuVram), modelName, modelVariant, executionDType, fmt.Sprint(quantizeBits), fmt.Sprint(minVRAM), fmt.Sprint(maxVRAM)}
	GPUExecutionSDOverheadSeconds.WithLabelValues(labels...).Set(sdOverhead)
	GPUExecutionSecondsPerSDPixelStep.WithLabelValues(labels...).Set(sdRate)
	for i, name := range []string{"constant", "text_input", "output", "model_switch", "image_count", "image_megapixel"} {
		GPUExecutionLLMCoefficient.WithLabelValues(append([]string{name}, labels...)...).Set(llm[i])
	}
	GPUExecutionCalibrationSamples.WithLabelValues(labels...).Set(float64(samples))
}

// ModelNodeCount is one relay_model_nodes entry: the distinct node counts of
// one huggingface base model.
type ModelNodeCount struct {
	HFModelID string
	OnDisk    int64
	InMemory  int64
}

// SetModelNodes replaces the relay_model_nodes gauge series with the given
// entries. Models absent from entries are removed so the gauge always
// reflects the latest top-model snapshot.
func SetModelNodes(entries []ModelNodeCount) {
	ModelNodes.Reset()
	for _, entry := range entries {
		ModelNodes.WithLabelValues(entry.HFModelID, "on_disk").Set(float64(entry.OnDisk))
		ModelNodes.WithLabelValues(entry.HFModelID, "in_memory").Set(float64(entry.InMemory))
	}
}

// SelectionLabels is the label tuple used by node selection metrics.
type SelectionLabels struct {
	TaskType string
	VramTier string
	GPU      string
}

// SetNodeSelectionEmptyPoolTasks replaces the empty-pool gauge series with
// the counts observed in the latest matching round. Series from previous
// rounds that are absent from counts are removed so each task class reports
// its current number of unmatchable queued tasks.
func SetNodeSelectionEmptyPoolTasks(counts map[SelectionLabels]int) {
	NodeSelectionEmptyPoolTasks.Reset()
	for labels, count := range counts {
		NodeSelectionEmptyPoolTasks.WithLabelValues(labels.TaskType, labels.VramTier, labels.GPU).Set(float64(count))
	}
}

var vramTiers []uint64

// InitVramTiers stores the configured VRAM tier boundaries (in GB, ascending)
// used to map raw task VRAM requirements to low-cardinality tier labels.
func InitVramTiers(tiers []uint64) {
	vramTiers = append([]uint64(nil), tiers...)
	sort.Slice(vramTiers, func(i, j int) bool { return vramTiers[i] < vramTiers[j] })
}

// InitTaskExecutionTimeoutBuckets creates and registers
// relay_task_execution_timeout_seconds with the given ascending bucket upper
// bounds in seconds. A previous registration of the same metric is replaced.
func InitTaskExecutionTimeoutBuckets(buckets []uint64) {
	floatBuckets := make([]float64, len(buckets))
	for i, b := range buckets {
		floatBuckets[i] = float64(b)
	}
	m := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "relay_task_execution_timeout_seconds",
		Help:    "Execution-stage Timeout written on the task at dispatch (TaskStarted).",
		Buckets: floatBuckets,
	}, []string{"task_type", "vram_tier"})
	if TaskExecutionTimeoutSeconds != nil {
		Registry.Unregister(TaskExecutionTimeoutSeconds)
	}
	TaskExecutionTimeoutSeconds = m
	Registry.MustRegister(TaskExecutionTimeoutSeconds)
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
	case models.TaskAbortGroupTimeout:
		return "group_timeout"
	case models.TaskAbortErrorReported:
		return "error_reported"
	case models.TaskAbortCreatorCancelled:
		return "creator_cancelled"
	case models.TaskAbortCreatorValidationTimeout:
		return "creator_validation_timeout"
	case models.TaskAbortResultUploadTimeout:
		return "result_upload_timeout"
	case models.TaskAbortNodeSlashed:
		return "node_slashed"
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
