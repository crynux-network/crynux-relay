package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"errors"
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/mat"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gpuCalibrationKey struct {
	name string
	vram uint64
}

type taskPricingKey struct {
	taskType   models.TaskType
	vramDemand uint64
}

type executionParameters struct {
	sdRate float64
	llm    [3]float64
}

type cachedGPUCalibration struct {
	record       models.GPUExecutionCalibration
	dirtyVersion uint64
}

type executionGPUSnapshot struct {
	GPUName    string
	GPUVram    uint64
	CapturedAt time.Time
}

type taskPricingStore struct {
	mu         sync.RWMutex
	records    map[gpuCalibrationKey]*cachedGPUCalibration
	aggregates map[taskPricingKey]executionParameters
	snapshots  map[string]executionGPUSnapshot
	version    uint64
}

var globalTaskPricing taskPricingStore

func initialExecutionParameters() executionParameters {
	cfg := config.GetConfig().TaskPricing
	return executionParameters{
		sdRate: cfg.InitialSecondsPerSDPixelStep,
		llm: [3]float64{
			cfg.InitialLLMConstantSeconds,
			cfg.InitialLLMSecondsPerInputByte,
			cfg.InitialLLMSecondsPerOutputToken,
		},
	}
}

func InitTaskPricing(ctx context.Context, db *gorm.DB) error {
	records, err := models.LoadGPUExecutionCalibrations(ctx, db)
	if err != nil {
		return err
	}
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	globalTaskPricing.records = make(map[gpuCalibrationKey]*cachedGPUCalibration, len(records))
	globalTaskPricing.aggregates = make(map[taskPricingKey]executionParameters)
	globalTaskPricing.snapshots = make(map[string]executionGPUSnapshot)
	globalTaskPricing.version = 0
	for i := range records {
		record := records[i]
		globalTaskPricing.records[gpuCalibrationKey{name: record.GPUName, vram: record.GPUVram}] = &cachedGPUCalibration{record: record}
	}
	publishTaskPricingMetricsLocked()
	return nil
}

func parametersFromRecord(record *models.GPUExecutionCalibration) executionParameters {
	return executionParameters{
		sdRate: record.SecondsPerSDPixelStep,
		llm: [3]float64{
			record.LLMConstantSeconds,
			record.LLMSecondsPerInputByte,
			record.LLMSecondsPerOutputToken,
		},
	}
}

func calibrationHasSamples(record *models.GPUExecutionCalibration, taskType models.TaskType) bool {
	if taskType == models.TaskTypeSD {
		return record.SDSuccessSamples > 0
	}
	return record.LLMSuccessSamples > 0
}

func aggregateParametersLocked(taskType models.TaskType, vram uint64, exactVRAM bool) executionParameters {
	var total executionParameters
	var count float64
	for key, cached := range globalTaskPricing.records {
		if exactVRAM {
			if key.vram != vram {
				continue
			}
		} else if key.vram < vram {
			continue
		}
		if !calibrationHasSamples(&cached.record, taskType) {
			continue
		}
		parameters := parametersFromRecord(&cached.record)
		if taskType == models.TaskTypeSD {
			total.sdRate += parameters.sdRate
		} else {
			for i := range total.llm {
				total.llm[i] += parameters.llm[i]
			}
		}
		count++
	}
	if count == 0 {
		return initialExecutionParameters()
	}
	if taskType == models.TaskTypeSD {
		total.sdRate /= count
	} else {
		for i := range total.llm {
			total.llm[i] /= count
		}
	}
	return total
}

func createExactCalibrationLocked(key gpuCalibrationKey) *cachedGPUCalibration {
	sd := aggregateParametersLocked(models.TaskTypeSD, key.vram, true)
	llm := aggregateParametersLocked(models.TaskTypeLLM, key.vram, true)
	cached := &cachedGPUCalibration{record: models.GPUExecutionCalibration{
		GPUName:                  key.name,
		GPUVram:                  key.vram,
		SecondsPerSDPixelStep:    sd.sdRate,
		LLMConstantSeconds:       llm.llm[0],
		LLMSecondsPerInputByte:   llm.llm[1],
		LLMSecondsPerOutputToken: llm.llm[2],
	}}
	globalTaskPricing.records[key] = cached
	return cached
}

func exactCalibrationLocked(gpuName string, gpuVram uint64) *cachedGPUCalibration {
	key := gpuCalibrationKey{name: gpuName, vram: gpuVram}
	if cached, ok := globalTaskPricing.records[key]; ok {
		return cached
	}
	return createExactCalibrationLocked(key)
}

func getTaskPricingParameters(task *models.InferenceTask) executionParameters {
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	if task.RequiredGPU != "" {
		parameters := parametersFromRecord(&exactCalibrationLocked(task.RequiredGPU, task.RequiredGPUVRAM).record)
		publishTaskPricingMetricsLocked()
		return parameters
	}
	key := taskPricingKey{taskType: task.TaskType, vramDemand: task.MinVRAM}
	if parameters, ok := globalTaskPricing.aggregates[key]; ok {
		return parameters
	}
	parameters := aggregateParametersLocked(task.TaskType, task.MinVRAM, false)
	globalTaskPricing.aggregates[key] = parameters
	publishTaskPricingMetricsLocked()
	return parameters
}

func CaptureTaskExecutionGPUSnapshot(taskIDCommitment, gpuName string, gpuVram uint64) {
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	globalTaskPricing.snapshots[taskIDCommitment] = executionGPUSnapshot{
		GPUName: gpuName, GPUVram: gpuVram, CapturedAt: time.Now(),
	}
}

func DeleteTaskExecutionGPUSnapshot(taskIDCommitment string) {
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	delete(globalTaskPricing.snapshots, taskIDCommitment)
}

// DeleteGroupRefundExecutionGPUSnapshots removes execution GPU snapshots for
// TaskEndGroupRefund members after the group can no longer supply verified
// completion tokens for LLM calibration.
func DeleteGroupRefundExecutionGPUSnapshots(ctx context.Context, db *gorm.DB, taskID string) {
	if taskID == "" {
		return
	}
	groupTasks, err := models.GetTaskGroupByTaskID(ctx, db, taskID)
	if err != nil {
		log.Errorf("TaskPricing: failed to load group refund snapshots for cleanup, task_id: %s, error: %v", taskID, err)
		return
	}
	for i := range groupTasks {
		if groupTasks[i].Status == models.TaskEndGroupRefund {
			DeleteTaskExecutionGPUSnapshot(groupTasks[i].TaskIDCommitment)
		}
	}
}

func TaskExecutionGPUSnapshot(taskIDCommitment string) (string, uint64, bool) {
	globalTaskPricing.mu.RLock()
	defer globalTaskPricing.mu.RUnlock()
	snapshot, ok := globalTaskPricing.snapshots[taskIDCommitment]
	return snapshot.GPUName, snapshot.GPUVram, ok
}

func markDirtyLocked(cached *cachedGPUCalibration) {
	globalTaskPricing.version++
	cached.dirtyVersion = globalTaskPricing.version
}

func recomputeAggregatesLocked(taskType models.TaskType, changedGPUVram uint64) {
	for key := range globalTaskPricing.aggregates {
		if key.taskType == taskType && key.vramDemand <= changedGPUVram {
			globalTaskPricing.aggregates[key] = aggregateParametersLocked(taskType, key.vramDemand, false)
		}
	}
}

func CalibrateValidatedSDTask(task *models.InferenceTask) error {
	if task.TaskType != models.TaskTypeSD || task.SDUnits == nil || *task.SDUnits == 0 ||
		!task.StartTime.Valid || !task.ScoreReadyTime.Valid {
		return nil
	}
	if task.Status != models.TaskValidated && task.Status != models.TaskGroupValidated &&
		task.Status != models.TaskEndGroupRefund {
		return nil
	}
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	snapshot, ok := globalTaskPricing.snapshots[task.TaskIDCommitment]
	if !ok {
		return nil
	}
	cached := exactCalibrationLocked(snapshot.GPUName, snapshot.GPUVram)
	sample := task.ScoreReadyTime.Time.Sub(task.StartTime.Time).Seconds() - config.GetConfig().TaskPricing.OverheadSeconds
	if sample < 0 {
		sample = 0
	}
	sample /= float64(*task.SDUnits)
	alpha := config.GetConfig().TaskPricing.CalibrationAlpha
	cached.record.SecondsPerSDPixelStep = alpha*sample + (1-alpha)*cached.record.SecondsPerSDPixelStep
	cached.record.SDSuccessSamples++
	markDirtyLocked(cached)
	recomputeAggregatesLocked(models.TaskTypeSD, snapshot.GPUVram)
	publishTaskPricingMetricsLocked()
	return nil
}

func llmXTX(record *models.GPUExecutionCalibration) *mat.SymDense {
	matrix := mat.NewSymDense(3, nil)
	matrix.SetSym(0, 0, record.LLMXTX00)
	matrix.SetSym(0, 1, record.LLMXTX01)
	matrix.SetSym(0, 2, record.LLMXTX02)
	matrix.SetSym(1, 1, record.LLMXTX11)
	matrix.SetSym(1, 2, record.LLMXTX12)
	matrix.SetSym(2, 2, record.LLMXTX22)
	return matrix
}

func llmMatrixFullRank(record *models.GPUExecutionCalibration) bool {
	var svd mat.SVD
	if !svd.Factorize(llmXTX(record), mat.SVDFull) {
		return false
	}
	return svd.Rank(1e-10) == 3
}

func fitLLMParameters(record *models.GPUExecutionCalibration) error {
	matrix := llmXTX(record)
	for i := 0; i < 3; i++ {
		matrix.SetSym(i, i, matrix.At(i, i)+config.GetConfig().TaskPricing.CalibrationRegularization)
	}
	y := mat.NewVecDense(3, []float64{record.LLMXTY0, record.LLMXTY1, record.LLMXTY2})
	var coefficients mat.VecDense
	if err := coefficients.SolveVec(matrix, y); err != nil {
		return err
	}
	values := [3]float64{coefficients.AtVec(0), coefficients.AtVec(1), coefficients.AtVec(2)}
	for i := range values {
		if math.IsNaN(values[i]) || math.IsInf(values[i], 0) {
			return errors.New("fitted LLM coefficient is not finite")
		}
		if values[i] < 0 {
			values[i] = 0
		}
	}
	record.LLMConstantSeconds = values[0]
	record.LLMSecondsPerInputByte = values[1]
	record.LLMSecondsPerOutputToken = values[2]
	return nil
}

func CalibrateUploadedLLMTask(task *models.InferenceTask, completionTokens uint64) error {
	if task.TaskType != models.TaskTypeLLM || task.LLMInputBytes == nil ||
		!task.StartTime.Valid || !task.ScoreReadyTime.Valid {
		return nil
	}
	if task.Status != models.TaskEndSuccess && task.Status != models.TaskEndGroupSuccess &&
		task.Status != models.TaskEndGroupRefund {
		return nil
	}
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	snapshot, ok := globalTaskPricing.snapshots[task.TaskIDCommitment]
	if !ok {
		return nil
	}
	cached := exactCalibrationLocked(snapshot.GPUName, snapshot.GPUVram)
	x := [3]float64{1, float64(*task.LLMInputBytes), float64(completionTokens)}
	actual := task.ScoreReadyTime.Time.Sub(task.StartTime.Time).Seconds()
	prediction := cached.record.LLMConstantSeconds +
		x[1]*cached.record.LLMSecondsPerInputByte +
		x[2]*cached.record.LLMSecondsPerOutputToken
	if prediction > 0 {
		maxActual := prediction * (1 + config.GetConfig().TaskPricing.CalibrationMaxPositiveResidualMultiple)
		if actual > maxActual {
			actual = maxActual
		}
	}
	alpha := config.GetConfig().TaskPricing.CalibrationAlpha
	decay := 1 - alpha
	next := cached.record
	next.LLMXTX00 = decay*next.LLMXTX00 + alpha*x[0]*x[0]
	next.LLMXTX01 = decay*next.LLMXTX01 + alpha*x[0]*x[1]
	next.LLMXTX02 = decay*next.LLMXTX02 + alpha*x[0]*x[2]
	next.LLMXTX11 = decay*next.LLMXTX11 + alpha*x[1]*x[1]
	next.LLMXTX12 = decay*next.LLMXTX12 + alpha*x[1]*x[2]
	next.LLMXTX22 = decay*next.LLMXTX22 + alpha*x[2]*x[2]
	next.LLMXTY0 = decay*next.LLMXTY0 + alpha*x[0]*actual
	next.LLMXTY1 = decay*next.LLMXTY1 + alpha*x[1]*actual
	next.LLMXTY2 = decay*next.LLMXTY2 + alpha*x[2]*actual
	next.LLMSuccessSamples++
	if err := fitLLMParameters(&next); err != nil {
		return err
	}
	cached.record = next
	markDirtyLocked(cached)
	recomputeAggregatesLocked(models.TaskTypeLLM, snapshot.GPUVram)
	publishTaskPricingMetricsLocked()
	return nil
}

func calibrationReady(record *models.GPUExecutionCalibration, taskType models.TaskType) bool {
	warmup := config.GetConfig().TaskPricing.CalibrationWarmupSuccessSamples
	if taskType == models.TaskTypeSD {
		return record.SDSuccessSamples >= warmup
	}
	return record.LLMSuccessSamples >= warmup && llmMatrixFullRank(record)
}

func ComputeExecutionTimeout(task *models.InferenceTask, gpuName string, gpuVram uint64) (uint64, error) {
	description, err := DescribeExecutionTimeout(task, gpuName, gpuVram)
	if err != nil {
		return 0, err
	}
	return description.ComputedTimeout, nil
}

type ExecutionTimeoutDescription struct {
	ColdStart                   bool
	PredictedExecutionSeconds   float64
	TimeoutMultiplier           float64
	MinExecutionTimeoutSeconds  uint64
	MaxExecutionTimeoutSeconds  uint64
	ComputedTimeout             uint64
}

// DescribeExecutionTimeout returns the cold-start flag, prediction, clamp inputs,
// and computed timeout for normal SD/LLM tasks. Non-relay-owned types return the
// stored Timeout without prediction fields.
func DescribeExecutionTimeout(task *models.InferenceTask, gpuName string, gpuVram uint64) (ExecutionTimeoutDescription, error) {
	cfg := config.GetConfig().TaskPricing
	description := ExecutionTimeoutDescription{
		TimeoutMultiplier:          cfg.TimeoutMultiplier,
		MinExecutionTimeoutSeconds: cfg.MinExecutionTimeoutSeconds,
		MaxExecutionTimeoutSeconds: cfg.MaxExecutionTimeoutSeconds,
		ComputedTimeout:            task.Timeout,
	}
	if task.TaskType != models.TaskTypeSD && task.TaskType != models.TaskTypeLLM {
		return description, nil
	}
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	target := exactCalibrationLocked(gpuName, gpuVram)
	var prediction float64
	var err error
	if calibrationReady(&target.record, task.TaskType) {
		description.ColdStart = false
		prediction, err = computeEstimatedNodeSeconds(task, parametersFromRecord(&target.record))
	} else {
		description.ColdStart = true
		prediction, err = computeEstimatedNodeSeconds(task, initialExecutionParameters())
		for key, cached := range globalTaskPricing.records {
			if err != nil || key.vram != gpuVram || key.name == gpuName || !calibrationReady(&cached.record, task.TaskType) {
				continue
			}
			candidate, candidateErr := computeEstimatedNodeSeconds(task, parametersFromRecord(&cached.record))
			if candidateErr != nil {
				return ExecutionTimeoutDescription{}, candidateErr
			}
			if candidate > prediction {
				prediction = candidate
			}
		}
	}
	if err != nil {
		return ExecutionTimeoutDescription{}, err
	}
	description.PredictedExecutionSeconds = prediction
	timeout := prediction * cfg.TimeoutMultiplier
	timeout = math.Max(timeout, float64(cfg.MinExecutionTimeoutSeconds))
	timeout = math.Min(timeout, float64(cfg.MaxExecutionTimeoutSeconds))
	description.ComputedTimeout = uint64(math.Ceil(timeout))
	return description, nil
}

func FlushTaskPricingCalibration(ctx context.Context, db *gorm.DB) error {
	type dirtyRecord struct {
		record  models.GPUExecutionCalibration
		version uint64
	}
	globalTaskPricing.mu.RLock()
	dirty := make([]dirtyRecord, 0)
	for _, cached := range globalTaskPricing.records {
		if cached.dirtyVersion != 0 {
			dirty = append(dirty, dirtyRecord{record: cached.record, version: cached.dirtyVersion})
		}
	}
	globalTaskPricing.mu.RUnlock()
	if len(dirty) == 0 {
		return nil
	}
	// The upsert resolves rows only by the (gpu_name, gpu_vram) unique key.
	// Primary keys and timestamps are stripped before the insert, and database
	// IDs are never copied back into the cache: batch-upsert ID reporting is
	// unreliable on MySQL, and a wrong cached ID would make a later flush
	// rewrite an unrelated row through a primary-key conflict.
	records := make([]models.GPUExecutionCalibration, len(dirty))
	for i := range dirty {
		records[i] = dirty[i].record
		records[i].ID = 0
		records[i].CreatedAt = time.Time{}
		records[i].UpdatedAt = time.Time{}
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "gpu_name"}, {Name: "gpu_vram"}},
		UpdateAll: true,
	}).Create(&records).Error
	if err != nil {
		return err
	}
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	for i := range dirty {
		key := gpuCalibrationKey{name: dirty[i].record.GPUName, vram: dirty[i].record.GPUVram}
		if cached, ok := globalTaskPricing.records[key]; ok && cached.dirtyVersion == dirty[i].version {
			cached.dirtyVersion = 0
		}
	}
	return nil
}

func CleanupTaskExecutionGPUSnapshots(now time.Time) int {
	cfg := config.GetConfig().TaskPricing
	maxAge := time.Duration(cfg.MaxExecutionTimeoutSeconds+
		cfg.AppValidationTimeoutSeconds+cfg.ResultUploadTimeoutSeconds) * time.Second
	cutoff := now.Add(-maxAge)
	globalTaskPricing.mu.Lock()
	defer globalTaskPricing.mu.Unlock()
	removed := 0
	for taskID, snapshot := range globalTaskPricing.snapshots {
		if snapshot.CapturedAt.Before(cutoff) {
			delete(globalTaskPricing.snapshots, taskID)
			removed++
		}
	}
	return removed
}

func StartTaskPricingCalibrationFlush(ctx context.Context, db *gorm.DB) {
	interval := time.Duration(config.GetConfig().TaskPricing.CalibrationFlushIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := FlushTaskPricingCalibration(flushCtx, db); err != nil {
				log.Errorf("TaskPricing: shutdown calibration flush failed: %v", err)
			}
			cancel()
			return
		case now := <-ticker.C:
			if err := FlushTaskPricingCalibration(ctx, db); err != nil && !errors.Is(err, context.Canceled) {
				log.Errorf("TaskPricing: calibration flush failed: %v", err)
			}
			CleanupTaskExecutionGPUSnapshots(now)
		}
	}
}

func publishTaskPricingMetricsLocked() {
	metrics.ResetTaskPricingCalibrationMetrics()
	for key, parameters := range globalTaskPricing.aggregates {
		metrics.SetTaskPricingCalibration(metrics.TaskTypeLabel(key.taskType), key.vramDemand, parameters.sdRate, parameters.llm)
	}
	for key, cached := range globalTaskPricing.records {
		metrics.SetGPUExecutionCalibration(
			key.name,
			key.vram,
			cached.record.SecondsPerSDPixelStep,
			[3]float64{
				cached.record.LLMConstantSeconds,
				cached.record.LLMSecondsPerInputByte,
				cached.record.LLMSecondsPerOutputToken,
			},
			cached.record.SDSuccessSamples,
			cached.record.LLMSuccessSamples,
		)
	}
}
