package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"errors"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGroupHasCreatorValidationTimeout(t *testing.T) {
	tasks := []*models.InferenceTask{
		newValidationGroupTask(models.TaskScoreReady, models.TaskAbortReasonNone, "node-a"),
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortCreatorValidationTimeout, "node-b"),
		newValidationGroupTask(models.TaskErrorReported, models.TaskAbortReasonNone, "node-c"),
	}
	if !groupHasCreatorValidationTimeout(tasks) {
		t.Fatal("expected creator validation timeout to block group validation")
	}
	tasks[1].AbortReason = models.TaskAbortTimeout
	if groupHasCreatorValidationTimeout(tasks) {
		t.Fatal("execution timeout must not block the existing group validation path")
	}
}

func TestRefreshValidationGroupReturnsAlreadyAppliedAfterConcurrentValidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatal(err)
	}
	const taskID = "0xvalidated"
	tasks := make([]*models.InferenceTask, 3)
	for i := range tasks {
		stored := models.InferenceTask{
			TaskIDCommitment: fmt.Sprintf("commitment-%d", i),
			TaskID:           taskID,
			Status:           models.TaskEndAborted,
		}
		if err := db.Create(&stored).Error; err != nil {
			t.Fatal(err)
		}
		stale := stored
		stale.TaskID = ""
		stale.Status = models.TaskScoreReady
		tasks[i] = &stale
	}
	_, err = refreshValidationGroupTasksForFinalUpdate(context.Background(), db, tasks, taskID)
	if !errors.Is(err, ErrValidationAlreadyApplied) {
		t.Fatalf("error = %v, want ErrValidationAlreadyApplied", err)
	}
}

func TestValidationGroupLockRejectsFreshCreatorValidationTimeoutWithoutTaskIDWrite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	tasks := make([]*models.InferenceTask, 3)
	for i := range tasks {
		nonce := []byte{byte(i + 2)}
		task := models.InferenceTask{
			TaskIDCommitment: crypto.Keccak256Hash(append([]byte{1}, nonce...)).Hex(),
			Nonce:            "0x0" + string(rune('2'+i)),
			Status:           models.TaskScoreReady,
		}
		if i == 1 {
			task.Status = models.TaskEndAborted
			task.AbortReason = models.TaskAbortCreatorValidationTimeout
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("failed to create task %d: %v", i, err)
		}

		staleTask := task
		staleTask.Status = models.TaskScoreReady
		staleTask.AbortReason = models.TaskAbortReasonNone
		tasks[i] = &staleTask
	}

	_, err = refreshValidationGroupTasksForFinalUpdate(context.Background(), db, tasks, "new-task-id")
	if err == nil || err.Error() != "task group validation expired" {
		t.Fatalf("expected fresh timeout state to reject validation, got %v", err)
	}

	var taskIDCount int64
	if err := db.Model(&models.InferenceTask{}).Where("task_id <> ''").Count(&taskIDCount).Error; err != nil {
		t.Fatalf("failed to count updated tasks: %v", err)
	}
	if taskIDCount != 0 {
		t.Fatalf("expected no task ID writes, got %d", taskIDCount)
	}
}

func TestRefreshValidationGroupTasksForFinalUpdateRejectsCreatorValidationTimeout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	taskID := "0xabc"
	tasks := make([]*models.InferenceTask, 3)
	for i := range tasks {
		task := models.InferenceTask{
			TaskIDCommitment: fmt.Sprintf("0xcommitment%d", i),
			Status:           models.TaskScoreReady,
			TaskID:           taskID,
			SelectedNode:     fmt.Sprintf("0xnode%d", i),
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("failed to create task %d: %v", i, err)
		}
		copied := task
		tasks[i] = &copied
	}

	if err := db.Model(&models.InferenceTask{}).Where("id = ?", tasks[1].ID).Updates(map[string]interface{}{
		"status":       models.TaskEndAborted,
		"abort_reason": models.TaskAbortCreatorValidationTimeout,
	}).Error; err != nil {
		t.Fatalf("failed to expire member: %v", err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := refreshValidationGroupTasksForFinalUpdate(context.Background(), tx, tasks, "new-task-id")
		return err
	})
	if err == nil || err.Error() != "task group validation expired" {
		t.Fatalf("expected final-update timeout recheck to reject, got %v", err)
	}

	var stored []models.InferenceTask
	if err := db.Order("id").Find(&stored).Error; err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(stored))
	}
	if stored[0].Status != models.TaskScoreReady || stored[2].Status != models.TaskScoreReady {
		t.Fatalf("non-expired members must keep original status: %+v %+v", stored[0], stored[2])
	}
	if stored[1].Status != models.TaskEndAborted || stored[1].AbortReason != models.TaskAbortCreatorValidationTimeout {
		t.Fatalf("expired member must remain creator-validation timeout: %+v", stored[1])
	}
	for _, task := range stored {
		if task.TaskID != taskID {
			t.Fatalf("task ID must remain unchanged, got %q", task.TaskID)
		}
		if task.QOSScore.Valid {
			t.Fatalf("qos must not be written on rejected final update: %+v", task.QOSScore)
		}
	}
}

func TestValidateTaskGroupRejectsWhenMemberAlreadyCreatorValidationTimeoutAfterTaskID(t *testing.T) {
	initServiceTestConfig(t)
	db := config.GetDB()
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}

	taskID := "0x01"
	originTasks := make([]*models.InferenceTask, 3)
	for i := range originTasks {
		nonce := []byte{byte(i + 2)}
		task := models.InferenceTask{
			TaskIDCommitment: crypto.Keccak256Hash(append([]byte{1}, nonce...)).Hex(),
			Nonce:            "0x0" + string(rune('2'+i)),
			Status:           models.TaskScoreReady,
			TaskID:           taskID,
			SelectedNode:     fmt.Sprintf("0xnode%d", i),
			Score:            "0xscore",
		}
		if i == 1 {
			task.Status = models.TaskEndAborted
			task.AbortReason = models.TaskAbortCreatorValidationTimeout
		}
		if err := db.Create(&task).Error; err != nil {
			t.Fatalf("failed to create task %d: %v", i, err)
		}
		copied := task
		originTasks[i] = &copied
	}

	err := ValidateTaskGroup(context.Background(), originTasks, taskID, "0xvrf", "0xpk")
	if err == nil || err.Error() != "task group validation expired" {
		t.Fatalf("expected ValidateTaskGroup to reject expired group, got %v", err)
	}

	var stored []models.InferenceTask
	if err := db.Order("id").Find(&stored).Error; err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}
	if stored[0].Status != models.TaskScoreReady || stored[2].Status != models.TaskScoreReady {
		t.Fatalf("remaining members must keep original status")
	}
	if stored[1].Status != models.TaskEndAborted || stored[1].AbortReason != models.TaskAbortCreatorValidationTimeout {
		t.Fatalf("expired member must remain unchanged")
	}
	for _, task := range stored {
		if task.Status == models.TaskGroupValidated ||
			task.Status == models.TaskEndGroupRefund ||
			task.Status == models.TaskEndInvalidated {
			t.Fatalf("rejected validation must not change member status to %+v", task.Status)
		}
		if task.QOSScore.Valid {
			t.Fatalf("rejected validation must not write qos")
		}
		if task.ValidatedTime.Valid {
			t.Fatalf("rejected validation must not write validated_time")
		}
	}
	for _, task := range originTasks {
		if task.QOSScore.Valid {
			t.Fatalf("rejected validation must not leave in-memory qos assigned")
		}
	}
}

func TestAssignValidationGroupQosScoresAllTimeoutTasksDoNotContribute(t *testing.T) {
	tasks := []*models.InferenceTask{
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortTimeout, "node-a"),
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortTimeout, "node-b"),
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortTimeout, "node-c"),
	}

	assignValidationGroupQosScores(tasks)

	for i, task := range tasks {
		if task.QOSScore.Valid {
			t.Fatalf("task %d should have invalid qos score when the whole group timed out", i)
		}
		if shouldPersistValidationGroupTimeoutQos(task) {
			t.Fatalf("task %d timeout score should not be persisted when the whole group timed out", i)
		}
	}
}

func TestAssignValidationGroupQosScoresTwoTimeoutsStillPenalizeLongTerm(t *testing.T) {
	tasks := []*models.InferenceTask{
		newValidationGroupTask(models.TaskScoreReady, models.TaskAbortReasonNone, "node-a"),
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortTimeout, "node-b"),
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortTimeout, "node-c"),
	}

	assignValidationGroupQosScores(tasks)

	if !tasks[0].QOSScore.Valid || tasks[0].QOSScore.Int64 != 10 {
		t.Fatalf("finished task should receive score 10, got %+v", tasks[0].QOSScore)
	}
	for i := 1; i < len(tasks); i++ {
		if !tasks[i].QOSScore.Valid || tasks[i].QOSScore.Int64 != 0 {
			t.Fatalf("timeout task %d should receive valid zero score, got %+v", i, tasks[i].QOSScore)
		}
		if !shouldPersistValidationGroupTimeoutQos(tasks[i]) {
			t.Fatalf("timeout task %d zero score should be persisted to long-term qos", i)
		}
	}
}

func TestAssignValidationGroupQosScoresSingleTimeoutStillPenalizesLongTerm(t *testing.T) {
	tasks := []*models.InferenceTask{
		newValidationGroupTask(models.TaskScoreReady, models.TaskAbortReasonNone, "node-a"),
		newValidationGroupTask(models.TaskEndGroupRefund, models.TaskAbortReasonNone, "node-b"),
		newValidationGroupTask(models.TaskEndAborted, models.TaskAbortTimeout, "node-c"),
	}

	assignValidationGroupQosScores(tasks)

	if !tasks[0].QOSScore.Valid || tasks[0].QOSScore.Int64 != 10 {
		t.Fatalf("first finished task should receive score 10, got %+v", tasks[0].QOSScore)
	}
	if !tasks[1].QOSScore.Valid || tasks[1].QOSScore.Int64 != 5 {
		t.Fatalf("second finished task should receive score 5, got %+v", tasks[1].QOSScore)
	}
	if !tasks[2].QOSScore.Valid || tasks[2].QOSScore.Int64 != 0 {
		t.Fatalf("timeout task should receive valid zero score, got %+v", tasks[2].QOSScore)
	}
	if !shouldPersistValidationGroupTimeoutQos(tasks[2]) {
		t.Fatalf("single timeout task zero score should be persisted to long-term qos")
	}
}

func newValidationGroupTask(status models.TaskStatus, abortReason models.TaskAbortReason, selectedNode string) *models.InferenceTask {
	return &models.InferenceTask{
		Status:       status,
		AbortReason:  abortReason,
		SelectedNode: selectedNode,
	}
}
