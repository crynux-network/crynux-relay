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
	llm    llmExecutionParameters
}

const llmFormulaVersion uint64 = 2

type llmExecutionParameters struct {
	constantSeconds       float64
	secondsPerInputByte   float64
	secondsPerOutputToken float64
	modelSwitchSeconds    float64
	secondsPerImage       float64
	secondsPerMegapixel   float64
}

func (p llmExecutionParameters) values() [6]float64 {
	return [6]float64{
		p.constantSeconds,
		p.secondsPerInputByte,
		p.secondsPerOutputToken,
		p.modelSwitchSeconds,
		p.secondsPerImage,
		p.secondsPerMegapixel,
	}
}

func llmParametersFromValues(values [6]float64) llmExecutionParameters {
	return llmExecutionParameters{
		constantSeconds:       values[0],
		secondsPerInputByte:   values[1],
		secondsPerOutputToken: values[2],
		modelSwitchSeconds:    values[3],
		secondsPerImage:       values[4],
		secondsPerMegapixel:   values[5],
	}
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
		llm: llmExecutionParameters{
			constantSeconds:       cfg.InitialLLMConstantSeconds,
			secondsPerInputByte:   cfg.InitialLLMSecondsPerInputByte,
			secondsPerOutputToken: cfg.InitialLLMSecondsPerOutputToken,
			modelSwitchSeconds:    cfg.InitialLLMModelSwitchSeconds,
			secondsPerImage:       cfg.InitialLLMSecondsPerImage,
			secondsPerMegapixel:   cfg.InitialLLMSecondsPerMegapixel,
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
		cached := &cachedGPUCalibration{record: record}
		if record.LLMFormulaVersion != llmFormulaVersion {
			resetLLMCalibration(&cached.record)
			globalTaskPricing.version++
			cached.dirtyVersion = globalTaskPricing.version
		}
		globalTaskPricing.records[gpuCalibrationKey{name: record.GPUName, vram: record.GPUVram}] = cached
	}
	publishTaskPricingMetricsLocked()
	return nil
}

func parametersFromRecord(record *models.GPUExecutionCalibration) executionParameters {
	return executionParameters{
		sdRate: record.SecondsPerSDPixelStep,
		llm: llmExecutionParameters{
			constantSeconds:       record.LLMConstantSeconds,
			secondsPerInputByte:   record.LLMSecondsPerInputByte,
			secondsPerOutputToken: record.LLMSecondsPerOutputToken,
			modelSwitchSeconds:    record.LLMModelSwitchSeconds,
			secondsPerImage:       record.LLMSecondsPerImage,
			secondsPerMegapixel:   record.LLMSecondsPerMegapixel,
		},
	}
}

func resetLLMCalibration(record *models.GPUExecutionCalibration) {
	initial := initialExecutionParameters().llm
	record.LLMConstantSeconds = initial.constantSeconds
	record.LLMSecondsPerInputByte = initial.secondsPerInputByte
	record.LLMSecondsPerOutputToken = initial.secondsPerOutputToken
	record.LLMModelSwitchSeconds = initial.modelSwitchSeconds
	record.LLMSecondsPerImage = initial.secondsPerImage
	record.LLMSecondsPerMegapixel = initial.secondsPerMegapixel
	record.LLMFormulaVersion = llmFormulaVersion
	record.LLMXTX00, record.LLMXTX01, record.LLMXTX02 = 0, 0, 0
	record.LLMXTX03, record.LLMXTX04, record.LLMXTX05 = 0, 0, 0
	record.LLMXTX11, record.LLMXTX12, record.LLMXTX13 = 0, 0, 0
	record.LLMXTX14, record.LLMXTX15, record.LLMXTX22 = 0, 0, 0
	record.LLMXTX23, record.LLMXTX24, record.LLMXTX25 = 0, 0, 0
	record.LLMXTX33, record.LLMXTX34, record.LLMXTX35 = 0, 0, 0
	record.LLMXTX44, record.LLMXTX45, record.LLMXTX55 = 0, 0, 0
	record.LLMXTY0, record.LLMXTY1, record.LLMXTY2 = 0, 0, 0
	record.LLMXTY3, record.LLMXTY4, record.LLMXTY5 = 0, 0, 0
	record.LLMSuccessSamples = 0
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
			totalValues := total.llm.values()
			parameterValues := parameters.llm.values()
			for i := range totalValues {
				totalValues[i] += parameterValues[i]
			}
			total.llm = llmParametersFromValues(totalValues)
		}
		count++
	}
	if count == 0 {
		return initialExecutionParameters()
	}
	if taskType == models.TaskTypeSD {
		total.sdRate /= count
	} else {
		values := total.llm.values()
		for i := range values {
			values[i] /= count
		}
		total.llm = llmParametersFromValues(values)
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
		LLMConstantSeconds:       llm.llm.constantSeconds,
		LLMSecondsPerInputByte:   llm.llm.secondsPerInputByte,
		LLMSecondsPerOutputToken: llm.llm.secondsPerOutputToken,
		LLMModelSwitchSeconds:    llm.llm.modelSwitchSeconds,
		LLMSecondsPerImage:       llm.llm.secondsPerImage,
		LLMSecondsPerMegapixel:   llm.llm.secondsPerMegapixel,
		LLMFormulaVersion:        llmFormulaVersion,
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
	matrix := mat.NewSymDense(6, nil)
	matrix.SetSym(0, 0, record.LLMXTX00)
	matrix.SetSym(0, 1, record.LLMXTX01)
	matrix.SetSym(0, 2, record.LLMXTX02)
	matrix.SetSym(0, 3, record.LLMXTX03)
	matrix.SetSym(0, 4, record.LLMXTX04)
	matrix.SetSym(0, 5, record.LLMXTX05)
	matrix.SetSym(1, 1, record.LLMXTX11)
	matrix.SetSym(1, 2, record.LLMXTX12)
	matrix.SetSym(1, 3, record.LLMXTX13)
	matrix.SetSym(1, 4, record.LLMXTX14)
	matrix.SetSym(1, 5, record.LLMXTX15)
	matrix.SetSym(2, 2, record.LLMXTX22)
	matrix.SetSym(2, 3, record.LLMXTX23)
	matrix.SetSym(2, 4, record.LLMXTX24)
	matrix.SetSym(2, 5, record.LLMXTX25)
	matrix.SetSym(3, 3, record.LLMXTX33)
	matrix.SetSym(3, 4, record.LLMXTX34)
	matrix.SetSym(3, 5, record.LLMXTX35)
	matrix.SetSym(4, 4, record.LLMXTX44)
	matrix.SetSym(4, 5, record.LLMXTX45)
	matrix.SetSym(5, 5, record.LLMXTX55)
	return matrix
}

func fitLLMParameters(record *models.GPUExecutionCalibration) error {
	matrix := llmXTX(record)
	regularization := config.GetConfig().TaskPricing.CalibrationRegularization
	initial := initialExecutionParameters().llm.values()
	yValues := [6]float64{
		record.LLMXTY0, record.LLMXTY1, record.LLMXTY2,
		record.LLMXTY3, record.LLMXTY4, record.LLMXTY5,
	}
	for i := 0; i < 6; i++ {
		matrix.SetSym(i, i, matrix.At(i, i)+regularization)
		yValues[i] += regularization * initial[i]
	}
	y := mat.NewVecDense(6, yValues[:])
	var coefficients mat.VecDense
	if err := coefficients.SolveVec(matrix, y); err != nil {
		return err
	}
	values := [6]float64{}
	for i := range values {
		values[i] = coefficients.AtVec(i)
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
	record.LLMModelSwitchSeconds = values[3]
	record.LLMSecondsPerImage = values[4]
	record.LLMSecondsPerMegapixel = values[5]
	record.LLMFormulaVersion = llmFormulaVersion
	return nil
}

func CalibrateUploadedLLMTask(task *models.InferenceTask, completionTokens uint64) error {
	if task.TaskType != models.TaskTypeLLM || task.LLMTextInputBytes == nil ||
		task.LLMImageCount == nil || task.LLMImagePixels == nil ||
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
	modelSwitched := 0.0
	if task.ModelSwtiched {
		modelSwitched = 1
	}
	x := [6]float64{
		1,
		float64(*task.LLMTextInputBytes),
		float64(completionTokens),
		modelSwitched,
		float64(*task.LLMImageCount),
		float64(*task.LLMImagePixels) / 1_000_000,
	}
	actual := task.ScoreReadyTime.Time.Sub(task.StartTime.Time).Seconds()
	parameterValues := parametersFromRecord(&cached.record).llm.values()
	prediction := 0.0
	for i := range x {
		prediction += x[i] * parameterValues[i]
	}
	if prediction > 0 {
		maxActual := prediction * (1 + config.GetConfig().TaskPricing.CalibrationMaxPositiveResidualMultiple)
		if actual > maxActual {
			actual = maxActual
		}
	}
	alpha := config.GetConfig().TaskPricing.CalibrationAlpha
	decay := 1 - alpha
	next := cached.record
	matrix := llmXTX(&next)
	yValues := [6]float64{
		next.LLMXTY0, next.LLMXTY1, next.LLMXTY2,
		next.LLMXTY3, next.LLMXTY4, next.LLMXTY5,
	}
	for i := range x {
		for j := i; j < len(x); j++ {
			matrix.SetSym(i, j, decay*matrix.At(i, j)+alpha*x[i]*x[j])
		}
		yValues[i] = decay*yValues[i] + alpha*x[i]*actual
	}
	setLLMFitState(&next, matrix, yValues)
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

func setLLMFitState(record *models.GPUExecutionCalibration, matrix *mat.SymDense, y [6]float64) {
	record.LLMXTX00, record.LLMXTX01, record.LLMXTX02 = matrix.At(0, 0), matrix.At(0, 1), matrix.At(0, 2)
	record.LLMXTX03, record.LLMXTX04, record.LLMXTX05 = matrix.At(0, 3), matrix.At(0, 4), matrix.At(0, 5)
	record.LLMXTX11, record.LLMXTX12 = matrix.At(1, 1), matrix.At(1, 2)
	record.LLMXTX13, record.LLMXTX14, record.LLMXTX15 = matrix.At(1, 3), matrix.At(1, 4), matrix.At(1, 5)
	record.LLMXTX22, record.LLMXTX23 = matrix.At(2, 2), matrix.At(2, 3)
	record.LLMXTX24, record.LLMXTX25 = matrix.At(2, 4), matrix.At(2, 5)
	record.LLMXTX33, record.LLMXTX34, record.LLMXTX35 = matrix.At(3, 3), matrix.At(3, 4), matrix.At(3, 5)
	record.LLMXTX44, record.LLMXTX45, record.LLMXTX55 = matrix.At(4, 4), matrix.At(4, 5), matrix.At(5, 5)
	record.LLMXTY0, record.LLMXTY1, record.LLMXTY2 = y[0], y[1], y[2]
	record.LLMXTY3, record.LLMXTY4, record.LLMXTY5 = y[3], y[4], y[5]
}

func calibrationReady(record *models.GPUExecutionCalibration, taskType models.TaskType) bool {
	warmup := config.GetConfig().TaskPricing.CalibrationWarmupSuccessSamples
	if taskType == models.TaskTypeSD {
		return record.SDSuccessSamples >= warmup
	}
	return record.LLMSuccessSamples >= warmup && record.LLMFormulaVersion == llmFormulaVersion
}

func ComputeExecutionTimeout(task *models.InferenceTask, gpuName string, gpuVram uint64, switched ...bool) (uint64, error) {
	timeoutTask := *task
	if len(switched) > 0 {
		timeoutTask.ModelSwtiched = switched[0]
	}
	description, err := DescribeExecutionTimeout(&timeoutTask, gpuName, gpuVram)
	if err != nil {
		return 0, err
	}
	return description.ComputedTimeout, nil
}

type ExecutionTimeoutDescription struct {
	ColdStart                  bool
	PredictedExecutionSeconds  float64
	TimeoutMultiplier          float64
	MinExecutionTimeoutSeconds uint64
	MaxExecutionTimeoutSeconds uint64
	ComputedTimeout            uint64
	ConstantSeconds            float64
	TextInputSeconds           float64
	OutputTokenSeconds         float64
	ModelSwitchSeconds         float64
	ImageCountSeconds          float64
	ImageMegapixelSeconds      float64
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
		parameters := parametersFromRecord(&target.record)
		prediction, err = computeEstimatedNodeSeconds(task, parameters, task.ModelSwtiched)
		setLLMTimeoutContributions(&description, task, parameters.llm)
	} else {
		description.ColdStart = true
		selectedParameters := initialExecutionParameters()
		prediction, err = computeEstimatedNodeSeconds(task, selectedParameters, task.ModelSwtiched)
		for key, cached := range globalTaskPricing.records {
			if err != nil || key.vram != gpuVram || key.name == gpuName || !calibrationReady(&cached.record, task.TaskType) {
				continue
			}
			candidateParameters := parametersFromRecord(&cached.record)
			candidate, candidateErr := computeEstimatedNodeSeconds(task, candidateParameters, task.ModelSwtiched)
			if candidateErr != nil {
				return ExecutionTimeoutDescription{}, candidateErr
			}
			if candidate > prediction {
				prediction = candidate
				selectedParameters = candidateParameters
			}
		}
		setLLMTimeoutContributions(&description, task, selectedParameters.llm)
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

func setLLMTimeoutContributions(description *ExecutionTimeoutDescription, task *models.InferenceTask, parameters llmExecutionParameters) {
	if task.TaskType != models.TaskTypeLLM || task.LLMTextInputBytes == nil || task.LLMMaxNewTokens == nil ||
		task.LLMImageCount == nil || task.LLMImagePixels == nil {
		return
	}
	description.ConstantSeconds = parameters.constantSeconds
	description.TextInputSeconds = float64(*task.LLMTextInputBytes) * parameters.secondsPerInputByte
	description.OutputTokenSeconds = float64(*task.LLMMaxNewTokens) * parameters.secondsPerOutputToken
	if task.ModelSwtiched {
		description.ModelSwitchSeconds = parameters.modelSwitchSeconds
	}
	description.ImageCountSeconds = float64(*task.LLMImageCount) * parameters.secondsPerImage
	description.ImageMegapixelSeconds = float64(*task.LLMImagePixels) / 1_000_000 * parameters.secondsPerMegapixel
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
		metrics.SetTaskPricingCalibration(metrics.TaskTypeLabel(key.taskType), key.vramDemand, parameters.sdRate, parameters.llm.values())
	}
	for key, cached := range globalTaskPricing.records {
		metrics.SetGPUExecutionCalibration(
			key.name,
			key.vram,
			cached.record.SecondsPerSDPixelStep,
			parametersFromRecord(&cached.record).llm.values(),
			cached.record.SDSuccessSamples,
			cached.record.LLMSuccessSamples,
		)
	}
}
