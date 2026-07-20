package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	log "github.com/sirupsen/logrus"
	"github.com/vechain/go-ecvrf"
	"gorm.io/gorm"
)

func validateTaskID(taskID, nonce, taskIDCommitment string) error {
	taskIDBytes, err := hexutil.Decode(taskID)
	if err != nil {
		return err
	}
	nonceBytes, err := hexutil.Decode(nonce)
	if err != nil {
		return err
	}

	hash := crypto.Keccak256Hash(append(taskIDBytes, nonceBytes...))
	if hash.Hex() != taskIDCommitment {
		return errors.New("task id incorrect")
	}
	return nil
}

func validateVRFProof(samplingSeed, vrfProof, publicKey string, creator string, grouped bool) error {
	samplingSeedBytes, err := hexutil.Decode(samplingSeed)
	if err != nil {
		return errors.New("invalid sampling seed")
	}

	pkBytes, err := hexutil.Decode(publicKey)
	if err != nil {
		return errors.New("invalid public key")
	}
	if len(pkBytes) != 64 {
		return errors.New("invalid public key")
	}
	vrfProofBytes, err := hexutil.Decode(vrfProof)
	if err != nil {
		return errors.New("invalid vrf proof")
	}
	if len(vrfProofBytes) != 81 {
		return errors.New("invalid vrf proof")
	}

	pkBytes = append([]byte{0x04}, pkBytes...)
	pubKey, err := secp256k1.ParsePubKey(pkBytes)
	if err != nil {
		return err
	}
	ecdsaPubKey := pubKey.ToECDSA()
	address := crypto.PubkeyToAddress(*ecdsaPubKey)
	if address.Hex() != creator {
		return errors.New("not task creator")
	}
	beta, err := ecvrf.Secp256k1Sha256Tai.Verify(pubKey.ToECDSA(), samplingSeedBytes, vrfProofBytes)
	if err != nil {
		return err
	}
	needValidation := utils.VrfNeedValidation(beta)
	if grouped && !needValidation {
		return errors.New("task is not selected for validation")
	}
	if !grouped && needValidation {
		return errors.New("task is selected for validation")
	}
	return nil
}

func ValidateSingleTask(ctx context.Context, originTask *models.InferenceTask, taskID, vrfProof, publicKey string) error {
	task := *originTask
	if !(task.Status == models.TaskScoreReady || task.Status == models.TaskErrorReported) {
		return errors.New("illegal task state")
	}

	if err := validateTaskID(taskID, task.Nonce, task.TaskIDCommitment); err != nil {
		return err
	}
	if err := task.Update(ctx, config.GetDB(), map[string]interface{}{"task_id": taskID}); err != nil {
		return err
	}

	if err := validateVRFProof(task.SamplingSeed, vrfProof, publicKey, task.Creator, false); err != nil {
		return err
	}

	var err error
	if task.Status == models.TaskScoreReady {
		err = SetTaskStatusValidated(ctx, config.GetDB(), &task)
	} else {
		task.AbortReason = models.TaskAbortErrorReported
		task.ValidatedTime = sql.NullTime{Time: time.Now(), Valid: true}
		err = ExecuteNodeStateUpdate(ctx, config.GetDB(), []string{task.SelectedNode}, func() error {
			return SetTaskStatusEndAborted(ctx, config.GetDB(), &task, task.Creator)
		})
	}
	if err != nil {
		return err
	}
	*originTask = task
	return nil
}

func checkHammingDistance(h1, h2 []byte, threshold uint64) bool {
	if len(h1) != len(h2) || len(h1)%8 != 0 {
		return false
	}

	for i := 0; i < len(h1); i += 8 {
		distance := utils.HammingDistance(h1[i:i+8], h2[i:i+8])
		if uint64(distance) >= threshold {
			return false
		}
	}

	return true
}

func compareTaskScore(task1, task2 *models.InferenceTask, threshold uint64) bool {
	if task1.TaskType != task2.TaskType {
		return false
	}
	if task1.Status != task2.Status {
		return false
	}
	if task1.Status == models.TaskScoreReady {
		switch task1.TaskType {
		case models.TaskTypeSD, models.TaskTypeSDFTLora:
			h1 := hexutil.MustDecode(task1.Score)
			h2 := hexutil.MustDecode(task2.Score)
			return checkHammingDistance(h1, h2, threshold)
		case models.TaskTypeLLM:
			return task1.Score == task2.Score
		default:
			return false
		}
	} else {
		return true
	}
}

func assignValidationGroupQosScores(tasks []*models.InferenceTask) {
	order := 0
	finishedTaskCount := 0
	for _, task := range tasks {
		if task.Status != models.TaskEndAborted {
			score := getTaskQosScore(order)
			task.QOSScore = sql.NullInt64{Int64: int64(score), Valid: true}
			order++
			finishedTaskCount++
			continue
		}
		task.QOSScore = sql.NullInt64{Int64: 0, Valid: true}
	}
	if finishedTaskCount == 0 {
		for _, task := range tasks {
			task.QOSScore = sql.NullInt64{Int64: 0, Valid: false}
		}
	}
}

func shouldPersistValidationGroupTimeoutQos(task *models.InferenceTask) bool {
	return task.Status == models.TaskEndAborted &&
		task.AbortReason == models.TaskAbortTimeout &&
		task.QOSScore.Valid &&
		len(task.SelectedNode) > 0
}

func persistValidationGroupAbortedTaskQos(ctx context.Context, tx *gorm.DB, task *models.InferenceTask) error {
	if shouldPersistValidationGroupTimeoutQos(task) {
		node, err := models.GetNodeByAddress(ctx, tx, task.SelectedNode)
		if err != nil {
			return err
		}
		if err := updateNodeQosScore(ctx, tx, node, uint64(task.QOSScore.Int64)); err != nil {
			return err
		}
	}
	return task.Update(ctx, tx, map[string]interface{}{
		"qos_score": task.QOSScore,
	})
}

type prebuiltSlashEvidence struct {
	evidence         *models.SlashEvidence
	evidenceComplete bool
}

func buildInvalidatedTaskSlashEvidence(ctx context.Context, db *gorm.DB, tasks []*models.InferenceTask, nextStatusMap map[string]models.TaskStatus) (map[string]prebuiltSlashEvidence, error) {
	evidenceByTaskIDCommitment := make(map[string]prebuiltSlashEvidence)
	for _, task := range tasks {
		if nextStatusMap[task.TaskIDCommitment] != models.TaskEndInvalidated {
			continue
		}
		node, err := checkTaskSelectedNode(ctx, db, task)
		if err != nil {
			return nil, err
		}
		evidence, evidenceComplete, err := buildSlashEvidence(ctx, db, task, node)
		if err != nil {
			return nil, err
		}
		evidenceByTaskIDCommitment[task.TaskIDCommitment] = prebuiltSlashEvidence{
			evidence:         evidence,
			evidenceComplete: evidenceComplete,
		}
	}
	return evidenceByTaskIDCommitment, nil
}

func ValidateTaskGroup(ctx context.Context, originTasks []*models.InferenceTask, taskID, vrfProof, publicKey string) error {
	tasks := make([]*models.InferenceTask, len(originTasks))
	for i, task := range originTasks {
		newTask := *task
		tasks[i] = &newTask
	}

	if len(tasks) != 3 {
		return errors.New("task group size is not 3")
	}

	for _, task := range tasks {
		if !(task.Status == models.TaskScoreReady || task.Status == models.TaskErrorReported || task.Status == models.TaskEndAborted) {
			return errors.New("illegal task state")
		}
	}

	for _, task := range tasks {
		if err := validateTaskID(taskID, task.Nonce, task.TaskIDCommitment); err != nil {
			return err
		}
	}
	if err := config.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			if err := task.Update(ctx, tx, map[string]interface{}{"task_id": taskID}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// sort tasks by sequence
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
	// validate vrf proof
	samplingSeed := tasks[0].SamplingSeed
	for _, task := range tasks {
		if err := validateVRFProof(samplingSeed, vrfProof, publicKey, task.Creator, true); err != nil {
			return err
		}
	}

	// sort tasks by time cost
	sort.Slice(tasks, func(i, j int) bool {
		ti := tasks[i].ExecutionTime()
		tj := tasks[j].ExecutionTime()
		if ti == tj {
			return tasks[i].ID < tasks[j].ID
		}
		return ti < tj
	})
	// set task qos score
	assignValidationGroupQosScores(tasks)

	shouldLogValidationGroup := config.GetTaskValidationGroupLogger() != nil
	var validationGroupStatusLabels []string
	validationGroupNodeMetricsBefore := make(map[string]validationGroupNodeMetrics)
	validationGroupNodeOrder := make([]string, 0, len(tasks))
	if shouldLogValidationGroup {
		validationGroupStatusLabels = collectValidationGroupStatusLabels(tasks)
		nodeMetricsBefore, orderedNodeAddresses, err := collectValidationGroupNodeMetricsBefore(ctx, tasks)
		if err != nil {
			log.Errorf("TaskValidationGroup: collect pre-update node metrics error: %v", err)
		} else {
			validationGroupNodeMetricsBefore = nodeMetricsBefore
			validationGroupNodeOrder = orderedNodeAddresses
		}
	}

	// validate tasks' score
	appConfig := config.GetConfig()

	nextStatusMap := make(map[string]models.TaskStatus)
	finishedTasks := make([]*models.InferenceTask, 0)
	for _, task := range tasks {
		nextStatusMap[task.TaskIDCommitment] = models.TaskEndAborted
		if task.Status != models.TaskEndAborted {
			finishedTasks = append(finishedTasks, task)
		}
	}

	if len(finishedTasks) == 2 {
		if compareTaskScore(finishedTasks[0], finishedTasks[1], appConfig.Task.DistanceThreshold) && finishedTasks[0].Status == models.TaskScoreReady {
			nextStatusMap[finishedTasks[0].TaskIDCommitment] = models.TaskGroupValidated
			nextStatusMap[finishedTasks[1].TaskIDCommitment] = models.TaskEndGroupRefund
		}
	} else if len(finishedTasks) == 3 {
		same1 := compareTaskScore(finishedTasks[0], finishedTasks[1], appConfig.Task.DistanceThreshold)
		same2 := compareTaskScore(finishedTasks[0], finishedTasks[2], appConfig.Task.DistanceThreshold)
		same3 := compareTaskScore(finishedTasks[1], finishedTasks[2], appConfig.Task.DistanceThreshold)
		if same1 {
			if finishedTasks[0].Status == models.TaskScoreReady {
				nextStatusMap[finishedTasks[0].TaskIDCommitment] = models.TaskGroupValidated
				nextStatusMap[finishedTasks[1].TaskIDCommitment] = models.TaskEndGroupRefund
			}
			if same2 {
				nextStatusMap[finishedTasks[2].TaskIDCommitment] = models.TaskEndGroupRefund
			} else {
				nextStatusMap[finishedTasks[2].TaskIDCommitment] = models.TaskEndInvalidated
			}
		} else if same2 {
			if finishedTasks[0].Status == models.TaskScoreReady {
				nextStatusMap[finishedTasks[0].TaskIDCommitment] = models.TaskGroupValidated
				nextStatusMap[finishedTasks[2].TaskIDCommitment] = models.TaskEndGroupRefund
			}
			nextStatusMap[finishedTasks[1].TaskIDCommitment] = models.TaskEndInvalidated
		} else if same3 {
			if finishedTasks[1].Status == models.TaskScoreReady {
				nextStatusMap[finishedTasks[1].TaskIDCommitment] = models.TaskGroupValidated
				nextStatusMap[finishedTasks[2].TaskIDCommitment] = models.TaskEndGroupRefund
			}
			nextStatusMap[finishedTasks[0].TaskIDCommitment] = models.TaskEndInvalidated
		}
	}
	if shouldLogValidationGroup {
		markValidationGroupSlashedNodes(tasks, nextStatusMap, validationGroupNodeMetricsBefore)
		markValidationGroupKickoutCheckNodes(tasks, nextStatusMap, validationGroupNodeMetricsBefore)
	}

	slashEvidenceByTaskIDCommitment, err := buildInvalidatedTaskSlashEvidence(ctx, config.GetDB(), tasks, nextStatusMap)
	if err != nil {
		return err
	}

	groupNodeAddresses := make([]string, 0, len(tasks))
	for _, task := range tasks {
		groupNodeAddresses = append(groupNodeAddresses, task.SelectedNode)
	}
	if err := ExecuteNodeStateUpdate(ctx, config.GetDB(), groupNodeAddresses, func() error {
		return config.GetDB().Transaction(func(tx *gorm.DB) error {
			for _, task := range tasks {
				nextStatus := nextStatusMap[task.TaskIDCommitment]

				switch nextStatus {
				case models.TaskEndInvalidated:
					slashEvidence, ok := slashEvidenceByTaskIDCommitment[task.TaskIDCommitment]
					if !ok {
						return errors.New("missing prebuilt slash evidence")
					}
					if err := SetTaskStatusEndInvalidatedWithEvidence(ctx, tx, task, slashEvidence.evidence, slashEvidence.evidenceComplete); err != nil {
						return err
					}
				case models.TaskGroupValidated:
					if err := SetTaskStatusGroupValidated(ctx, tx, task); err != nil {
						return err
					}
				case models.TaskEndGroupRefund:
					if err := SetTaskStatusEndGroupRefund(ctx, tx, task); err != nil {
						return err
					}
				default:
					if task.Status != models.TaskEndAborted {
						// A finished task can only stay in the default aborted state for
						// two reasons: the rest of the group timed out before scoring
						// (fewer than 2 finished tasks, no comparison possible), or the
						// comparison ran and no majority was found.
						if len(finishedTasks) < 2 {
							task.AbortReason = models.TaskAbortGroupTimeout
						} else {
							task.AbortReason = models.TaskAbortIncorrectResult
						}

						task.ValidatedTime = sql.NullTime{Time: time.Now(), Valid: true}
						if err := SetTaskStatusEndAborted(ctx, tx, task, task.Creator); err != nil {
							return err
						}
					} else {
						if err := persistValidationGroupAbortedTaskQos(ctx, tx, task); err != nil {
							return err
						}
					}
				}
			}
			return nil
		})
	}); err != nil {
		return err
	}
	for i, task := range tasks {
		*originTasks[i] = *task
	}
	if shouldLogValidationGroup {
		nodeMetricsAfter, err := collectValidationGroupNodeMetricsAfter(ctx, validationGroupNodeMetricsBefore, validationGroupNodeOrder)
		if err != nil {
			log.Errorf("TaskValidationGroup: collect post-update node metrics error: %v", err)
			logValidationGroupEvent(taskID, tasks[0].TaskType, validationGroupStatusLabels, nil)
		} else {
			logValidationGroupEvent(taskID, tasks[0].TaskType, validationGroupStatusLabels, nodeMetricsAfter)
		}
	}
	return nil
}
