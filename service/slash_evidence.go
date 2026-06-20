package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	slashEvidenceReasonInvalidated = "task_end_invalidated"
	slashEvidenceStatusCopied      = "copied"
	slashEvidenceStatusMissing     = "missing"
	slashEvidenceStatusNotRequired = "not_required"
	slashEvidenceStatusPending     = "pending_upload"
	slashEvidenceStatusUploaded    = "uploaded"
)

var runningTaskSnapshots sync.Map

func captureRunningTaskSnapshot(ctx context.Context, db *gorm.DB, task *models.InferenceTask, node *models.Node) error {
	currentNode, err := models.GetNodeByAddress(ctx, db, node.Address)
	if err != nil {
		return err
	}
	snapshot, err := buildSlashEvidenceNodeSnapshot(ctx, db, currentNode)
	if err != nil {
		return err
	}
	runningTaskSnapshots.Store(task.TaskIDCommitment, snapshot)
	return nil
}

func deleteRunningTaskSnapshot(taskIDCommitment string) {
	runningTaskSnapshots.Delete(taskIDCommitment)
}

func buildSlashEvidence(ctx context.Context, db *gorm.DB, task *models.InferenceTask, node *models.Node) (*models.SlashEvidence, bool, error) {
	evidenceComplete := true
	var incompleteReason string

	groupTasks, err := getSlashEvidenceGroupTasks(ctx, db, task)
	if err != nil {
		return nil, false, err
	}
	inputArtifacts, err := copySlashEvidenceInputArtifacts(groupTasks)
	if err != nil {
		return nil, false, err
	}
	taskSnapshots, nodeSnapshots, snapshotsComplete, err := buildSlashEvidenceGroupSnapshots(ctx, db, groupTasks, task, node)
	if err != nil {
		return nil, false, err
	}
	if !snapshotsComplete {
		evidenceComplete = false
		incompleteReason = "one or more running task snapshots are missing"
	}
	groupTaskIDs, groupTaskIDCommitments := buildSlashEvidenceGroupContext(groupTasks)

	evidence := &models.SlashEvidence{
		TaskSnapshots: taskSnapshots,
		NodeSnapshots: nodeSnapshots,
		ValidationContext: models.SlashEvidenceValidationContext{
			Reason:            slashEvidenceReasonInvalidated,
			TaskID:            task.TaskID,
			GroupTaskIDs:      groupTaskIDs,
			TaskIDCommitments: groupTaskIDCommitments,
		},
		InputArtifacts:   inputArtifacts,
		ResultArtifacts:  buildPendingResultEvidenceArtifacts(groupTasks),
		IncompleteReason: incompleteReason,
	}
	return evidence, evidenceComplete, nil
}

func buildSlashEvidenceGroupSnapshots(ctx context.Context, db *gorm.DB, groupTasks []models.InferenceTask, invalidatedTask *models.InferenceTask, invalidatedNode *models.Node) ([]models.SlashEvidenceTaskSnapshot, []models.SlashEvidenceNodeSnapshot, bool, error) {
	taskSnapshots := make([]models.SlashEvidenceTaskSnapshot, 0, len(groupTasks))
	nodeSnapshots := make([]models.SlashEvidenceNodeSnapshot, 0, len(groupTasks))
	evidenceComplete := true
	for _, groupTask := range groupTasks {
		taskSnapshots = append(taskSnapshots, buildSlashEvidenceTaskSnapshot(&groupTask))
		nodeSnapshot, ok := loadRunningTaskNodeSnapshot(groupTask.TaskIDCommitment)
		if !ok {
			evidenceComplete = false
			node, err := getSlashEvidenceFallbackNode(ctx, db, &groupTask, invalidatedTask, invalidatedNode)
			if err != nil {
				return nil, nil, false, err
			}
			nodeSnapshot, err = buildSlashEvidenceNodeSnapshot(ctx, db, node)
			if err != nil {
				return nil, nil, false, err
			}
		}
		nodeSnapshots = append(nodeSnapshots, nodeSnapshot)
	}
	return taskSnapshots, nodeSnapshots, evidenceComplete, nil
}

func getSlashEvidenceFallbackNode(ctx context.Context, db *gorm.DB, task *models.InferenceTask, invalidatedTask *models.InferenceTask, invalidatedNode *models.Node) (*models.Node, error) {
	if task.TaskIDCommitment == invalidatedTask.TaskIDCommitment {
		return invalidatedNode, nil
	}
	return models.GetNodeByAddress(ctx, db, task.SelectedNode)
}

func loadRunningTaskNodeSnapshot(taskIDCommitment string) (models.SlashEvidenceNodeSnapshot, bool) {
	value, ok := runningTaskSnapshots.Load(taskIDCommitment)
	if !ok {
		return models.SlashEvidenceNodeSnapshot{}, false
	}
	snapshot, ok := value.(models.SlashEvidenceNodeSnapshot)
	return snapshot, ok
}

func buildSlashEvidenceTaskSnapshot(task *models.InferenceTask) models.SlashEvidenceTaskSnapshot {
	var qosScore *int64
	if task.QOSScore.Valid {
		value := task.QOSScore.Int64
		qosScore = &value
	}
	return models.SlashEvidenceTaskSnapshot{
		TaskIDCommitment: task.TaskIDCommitment,
		TaskID:           task.TaskID,
		TaskArgs:         task.TaskArgs,
		TaskType:         task.TaskType,
		TaskVersion:      task.TaskVersion,
		Creator:          task.Creator,
		TaskFee:          task.TaskFee.String(),
		Score:            task.Score,
		QOSScore:         qosScore,
		ModelIDs:         []string(task.ModelIDs),
		CreateTime:       nullTimePtr(task.CreateTime),
		StartTime:        nullTimePtr(task.StartTime),
		ScoreReadyTime:   nullTimePtr(task.ScoreReadyTime),
		ValidatedTime:    nullTimePtr(task.ValidatedTime),
	}
}

func buildSlashEvidenceNodeSnapshot(ctx context.Context, db *gorm.DB, node *models.Node) (models.SlashEvidenceNodeSnapshot, error) {
	modelsSnapshot := make([]models.SlashEvidenceNodeModel, 0, len(node.Models))
	nodeModels := node.Models
	if len(nodeModels) == 0 {
		var err error
		nodeModels, err = models.GetNodeModelsByNodeAddress(ctx, db, node.Address)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SlashEvidenceNodeSnapshot{}, err
		}
	}
	for _, nodeModel := range nodeModels {
		modelsSnapshot = append(modelsSnapshot, models.SlashEvidenceNodeModel{
			ModelID: nodeModel.ModelID,
			InUse:   nodeModel.InUse,
		})
	}

	delegatorCount, delegatedStakingAmount, err := getSlashEvidenceDelegationSummary(ctx, db, node.Address, node.Network)
	if err != nil {
		return models.SlashEvidenceNodeSnapshot{}, err
	}

	return models.SlashEvidenceNodeSnapshot{
		Address:                node.Address,
		Network:                node.Network,
		Status:                 node.Status,
		GPUName:                node.GPUName,
		GPUVram:                node.GPUVram,
		MajorVersion:           node.MajorVersion,
		MinorVersion:           node.MinorVersion,
		PatchVersion:           node.PatchVersion,
		OperatorStakeAmount:    node.StakeAmount.String(),
		QOSScore:               node.QOSScore,
		HealthBase:             node.HealthBase,
		HealthUpdatedAt:        nullTimePtr(node.HealthUpdatedAt),
		DelegatorCount:         delegatorCount,
		DelegatedStakingAmount: delegatedStakingAmount.String(),
		Models:                 modelsSnapshot,
	}, nil
}

func getSlashEvidenceDelegationSummary(ctx context.Context, db *gorm.DB, nodeAddress, network string) (int64, *big.Int, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var delegations []models.Delegation
	if err := db.WithContext(dbCtx).
		Where("node_address = ? AND network = ? AND slashed = ?", nodeAddress, network, false).
		Find(&delegations).Error; err != nil {
		return 0, nil, err
	}
	total := big.NewInt(0)
	for _, delegation := range delegations {
		total.Add(total, &delegation.Amount.Int)
	}
	return int64(len(delegations)), total, nil
}

func getSlashEvidenceGroupTasks(ctx context.Context, db *gorm.DB, task *models.InferenceTask) ([]models.InferenceTask, error) {
	if task.TaskID == "" {
		return []models.InferenceTask{*task}, nil
	}
	tasks, err := models.GetTaskGroupByTaskID(ctx, db, task.TaskID)
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func buildSlashEvidenceGroupContext(tasks []models.InferenceTask) ([]string, []string) {
	taskIDs := make([]string, 0, len(tasks))
	taskIDCommitments := make([]string, 0, len(tasks))
	for _, groupTask := range tasks {
		taskIDs = append(taskIDs, groupTask.TaskID)
		taskIDCommitments = append(taskIDCommitments, groupTask.TaskIDCommitment)
	}
	return taskIDs, taskIDCommitments
}

func copySlashEvidenceInputArtifacts(tasks []models.InferenceTask) ([]models.SlashEvidenceArtifacts, error) {
	artifacts := make([]models.SlashEvidenceArtifacts, 0, len(tasks))
	for _, task := range tasks {
		artifact, err := copySlashEvidenceInputArtifact(task.TaskIDCommitment)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func copySlashEvidenceInputArtifact(taskIDCommitment string) (models.SlashEvidenceArtifacts, error) {
	appConfig := config.GetConfig()
	sourcePath := filepath.Join(appConfig.DataDir.InferenceTasks, taskIDCommitment, "input")
	storedPath := filepath.Join(appConfig.DataDir.SlashedTasks, taskIDCommitment, "input")
	files, err := copyEvidenceDir(sourcePath, storedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return models.SlashEvidenceArtifacts{
				TaskIDCommitment: taskIDCommitment,
				SourcePath:       sourcePath,
				StoredPath:       storedPath,
				Status:           slashEvidenceStatusMissing,
			}, nil
		}
		return models.SlashEvidenceArtifacts{}, err
	}
	return models.SlashEvidenceArtifacts{
		TaskIDCommitment: taskIDCommitment,
		SourcePath:       sourcePath,
		StoredPath:       storedPath,
		Files:            files,
		Status:           slashEvidenceStatusCopied,
	}, nil
}

func buildPendingResultEvidenceArtifacts(tasks []models.InferenceTask) []models.SlashEvidenceArtifacts {
	artifacts := make([]models.SlashEvidenceArtifacts, 0, len(tasks))
	for _, task := range tasks {
		if task.Status == models.TaskEndGroupRefund {
			artifacts = append(artifacts, buildNotRequiredResultEvidenceArtifact(task.TaskIDCommitment))
			continue
		}
		artifacts = append(artifacts, buildPendingResultEvidenceArtifact(task.TaskIDCommitment))
	}
	return artifacts
}

func buildCurrentResultEvidenceArtifact(task models.InferenceTask) (models.SlashEvidenceArtifacts, error) {
	if task.Status == models.TaskEndGroupRefund {
		return buildNotRequiredResultEvidenceArtifact(task.TaskIDCommitment), nil
	}
	artifact, err := buildUploadedResultEvidenceArtifacts(task.TaskIDCommitment)
	if err != nil {
		if !os.IsNotExist(err) {
			return models.SlashEvidenceArtifacts{}, err
		}
		return buildPendingResultEvidenceArtifact(task.TaskIDCommitment), nil
	}
	return artifact, nil
}

func buildPendingResultEvidenceArtifact(taskIDCommitment string) models.SlashEvidenceArtifacts {
	appConfig := config.GetConfig()
	return models.SlashEvidenceArtifacts{
		TaskIDCommitment: taskIDCommitment,
		StoredPath:       filepath.Join(appConfig.DataDir.SlashedTasks, taskIDCommitment, "results"),
		Status:           slashEvidenceStatusPending,
	}
}

func buildNotRequiredResultEvidenceArtifact(taskIDCommitment string) models.SlashEvidenceArtifacts {
	return models.SlashEvidenceArtifacts{
		TaskIDCommitment: taskIDCommitment,
		Status:           slashEvidenceStatusNotRequired,
	}
}

func buildUploadedResultEvidenceArtifacts(taskIDCommitment string) (models.SlashEvidenceArtifacts, error) {
	appConfig := config.GetConfig()
	storedPath := filepath.Join(appConfig.DataDir.SlashedTasks, taskIDCommitment, "results")
	files, err := listEvidenceDirFiles(storedPath)
	if err != nil {
		return models.SlashEvidenceArtifacts{}, err
	}
	return models.SlashEvidenceArtifacts{
		TaskIDCommitment: taskIDCommitment,
		StoredPath:       storedPath,
		Files:            files,
		Status:           slashEvidenceStatusUploaded,
	}, nil
}

func copyEvidenceDir(sourcePath, storedPath string) ([]string, error) {
	if _, err := os.Stat(sourcePath); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(storedPath, 0o711); err != nil {
		return nil, err
	}
	files := make([]string, 0)
	if err := filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(storedPath, relativePath)
		if err := os.MkdirAll(filepath.Dir(destination), 0o711); err != nil {
			return err
		}
		if err := copyEvidenceFile(path, destination); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relativePath))
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

func listEvidenceDirFiles(path string) ([]string, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	files := make([]string, 0)
	if err := filepath.WalkDir(path, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(path, currentPath)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relativePath))
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

func copyEvidenceFile(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func nullTimePtr(value any) *time.Time {
	switch typedValue := value.(type) {
	case sql.NullTime:
		if !typedValue.Valid {
			return nil
		}
		return &typedValue.Time
	case time.Time:
		if typedValue.IsZero() {
			return nil
		}
		return &typedValue
	case interface{ IsZero() bool }:
		if typedValue.IsZero() {
			return nil
		}
	}
	return nil
}

func createPendingSlash(ctx context.Context, db *gorm.DB, task *models.InferenceTask, node *models.Node, evidence *models.SlashEvidence, evidenceComplete bool) error {
	evidenceBytes, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	pendingSlash := &models.PendingSlash{
		Status:           models.PendingSlashStatusPending,
		NodeAddress:      node.Address,
		Network:          node.Network,
		TaskIDCommitment: task.TaskIDCommitment,
		EvidenceJSON:     string(evidenceBytes),
		EvidenceComplete: evidenceComplete,
	}
	return pendingSlash.Create(ctx, db)
}

func parseSlashEvidence(evidenceJSON string) (*models.SlashEvidence, error) {
	var evidence models.SlashEvidence
	if err := json.Unmarshal([]byte(evidenceJSON), &evidence); err != nil {
		return nil, err
	}
	return &evidence, nil
}

func ParsePendingSlashEvidence(pendingSlash *models.PendingSlash) (*models.SlashEvidence, error) {
	return parseSlashEvidence(pendingSlash.EvidenceJSON)
}

func UpdatePendingSlashResultEvidence(ctx context.Context, db *gorm.DB, taskIDCommitment string) error {
	task, err := models.GetTaskByIDCommitment(ctx, db, taskIDCommitment)
	if err != nil {
		return err
	}
	groupTasks, err := getSlashEvidenceGroupTasks(ctx, db, task)
	if err != nil {
		return err
	}
	groupTaskIDCommitments := make([]string, 0, len(groupTasks))
	resultArtifacts := make([]models.SlashEvidenceArtifacts, 0, len(groupTasks))
	for _, groupTask := range groupTasks {
		groupTaskIDCommitments = append(groupTaskIDCommitments, groupTask.TaskIDCommitment)
		artifact, err := buildCurrentResultEvidenceArtifact(groupTask)
		if err != nil {
			return err
		}
		resultArtifacts = append(resultArtifacts, artifact)
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var pendingSlashes []models.PendingSlash
	if err := db.WithContext(dbCtx).
		Where("task_id_commitment IN ?", groupTaskIDCommitments).
		Find(&pendingSlashes).Error; err != nil {
		return err
	}
	for i := range pendingSlashes {
		evidence, err := parseSlashEvidence(pendingSlashes[i].EvidenceJSON)
		if err != nil {
			return err
		}
		evidence.ResultArtifacts = resultArtifacts
		evidenceBytes, err := json.Marshal(evidence)
		if err != nil {
			return err
		}
		pendingSlashes[i].EvidenceJSON = string(evidenceBytes)
		if err := pendingSlashes[i].Save(ctx, db); err != nil {
			return err
		}
	}
	return nil
}
