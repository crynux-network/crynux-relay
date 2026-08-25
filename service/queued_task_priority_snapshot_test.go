package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func resetQueuedTaskPrioritySnapshotForTest() {
	queuedTaskPrioritySnapshotMutex.Lock()
	queuedTaskPrioritySnapshot = QueuedTaskPrioritySnapshot{}
	queuedTaskPrioritySnapshotMutex.Unlock()
}

func newQueuedPrioritySnapshotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("failed to migrate inference tasks: %v", err)
	}
	return db
}

func createQueuedPriorityTask(t *testing.T, db *gorm.DB, taskType models.TaskType, priority string, commitment string) {
	t.Helper()
	z := new(big.Int)
	if _, ok := z.SetString(priority, 10); !ok {
		t.Fatalf("invalid priority %q", priority)
	}
	task := models.InferenceTask{
		TaskIDCommitment: commitment,
		Status:           models.TaskQueued,
		TaskType:         taskType,
		Priority:         models.BigInt{Int: *z},
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
}

func TestRefreshQueuedTaskPrioritySnapshotConvertsToIntegerGwei(t *testing.T) {
	resetQueuedTaskPrioritySnapshotForTest()
	db := newQueuedPrioritySnapshotTestDB(t)
	createQueuedPriorityTask(t, db, models.TaskTypeLLM, "57447899546", "high")
	createQueuedPriorityTask(t, db, models.TaskTypeSD, "33486159717", "mid")
	createQueuedPriorityTask(t, db, models.TaskTypeLLM, "25314079597", "low")

	now := time.Unix(1_700_000_000, 0).UTC()
	if err := RefreshQueuedTaskPrioritySnapshot(context.Background(), db, now); err != nil {
		t.Fatalf("RefreshQueuedTaskPrioritySnapshot: %v", err)
	}

	snapshot := GetQueuedTaskPrioritySnapshot()
	if snapshot.AsOf != now.Unix() {
		t.Fatalf("expected as_of %d, got %d", now.Unix(), snapshot.AsOf)
	}
	if snapshot.QueuedTaskCount != 3 {
		t.Fatalf("expected count 3, got %d", snapshot.QueuedTaskCount)
	}
	if snapshot.HighestPriorityGwei.String() != "57" {
		t.Fatalf("expected highest 57, got %s", snapshot.HighestPriorityGwei.String())
	}
	if snapshot.MedianPriorityGwei.String() != "33" {
		t.Fatalf("expected median 33, got %s", snapshot.MedianPriorityGwei.String())
	}
	if snapshot.LowestPriorityGwei.String() != "25" {
		t.Fatalf("expected lowest 25, got %s", snapshot.LowestPriorityGwei.String())
	}
}

func TestRefreshQueuedTaskPrioritySnapshotEmptyQueue(t *testing.T) {
	resetQueuedTaskPrioritySnapshotForTest()
	db := newQueuedPrioritySnapshotTestDB(t)
	now := time.Unix(1_700_000_100, 0).UTC()
	if err := RefreshQueuedTaskPrioritySnapshot(context.Background(), db, now); err != nil {
		t.Fatalf("RefreshQueuedTaskPrioritySnapshot: %v", err)
	}
	snapshot := GetQueuedTaskPrioritySnapshot()
	if snapshot.AsOf != now.Unix() || snapshot.QueuedTaskCount != 0 {
		t.Fatalf("unexpected snapshot %+v", snapshot)
	}
	if snapshot.HighestPriorityGwei != nil || snapshot.MedianPriorityGwei != nil || snapshot.LowestPriorityGwei != nil {
		t.Fatalf("expected null priorities, got %+v", snapshot)
	}
}

func TestRefreshQueuedTaskPrioritySnapshotKeepsPreviousOnFailure(t *testing.T) {
	resetQueuedTaskPrioritySnapshotForTest()
	db := newQueuedPrioritySnapshotTestDB(t)
	createQueuedPriorityTask(t, db, models.TaskTypeLLM, "2000000000", "only")
	firstAsOf := time.Unix(1_700_000_200, 0).UTC()
	if err := RefreshQueuedTaskPrioritySnapshot(context.Background(), db, firstAsOf); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	secondAsOf := time.Unix(1_700_000_300, 0).UTC()
	if err := RefreshQueuedTaskPrioritySnapshot(context.Background(), db, secondAsOf); err == nil {
		t.Fatal("expected refresh failure after db close")
	}

	snapshot := GetQueuedTaskPrioritySnapshot()
	if snapshot.AsOf != firstAsOf.Unix() {
		t.Fatalf("expected previous as_of %d, got %d", firstAsOf.Unix(), snapshot.AsOf)
	}
	if snapshot.QueuedTaskCount != 1 || snapshot.HighestPriorityGwei.String() != "2" {
		t.Fatalf("expected previous snapshot retained, got %+v", snapshot)
	}
}

func TestGetQueuedTaskPrioritySnapshotReturnsCopy(t *testing.T) {
	resetQueuedTaskPrioritySnapshotForTest()
	db := newQueuedPrioritySnapshotTestDB(t)
	createQueuedPriorityTask(t, db, models.TaskTypeLLM, "5000000000", "copy")
	if err := RefreshQueuedTaskPrioritySnapshot(context.Background(), db, time.Unix(1_700_000_400, 0).UTC()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	first := GetQueuedTaskPrioritySnapshot()
	first.HighestPriorityGwei.Add(&first.HighestPriorityGwei.Int, big.NewInt(1))
	second := GetQueuedTaskPrioritySnapshot()
	if second.HighestPriorityGwei.String() != "5" {
		t.Fatalf("expected getter copy to protect snapshot, got %s", second.HighestPriorityGwei.String())
	}
}

func TestQueuedTaskPrioritySnapshotConcurrentReadRefresh(t *testing.T) {
	resetQueuedTaskPrioritySnapshotForTest()
	db := newQueuedPrioritySnapshotTestDB(t)
	createQueuedPriorityTask(t, db, models.TaskTypeLLM, "1000000000", "race")
	if err := InitQueuedTaskPrioritySnapshot(context.Background(), db); err != nil {
		t.Fatalf("init: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 64)
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = GetQueuedTaskPrioritySnapshot()
		}()
		go func(i int) {
			defer wg.Done()
			if err := RefreshQueuedTaskPrioritySnapshot(context.Background(), db, time.Unix(int64(1_700_000_500+i), 0).UTC()); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("refresh error: %v", err)
		}
	}
	snapshot := GetQueuedTaskPrioritySnapshot()
	if snapshot.QueuedTaskCount != 1 || snapshot.HighestPriorityGwei.String() != "1" {
		t.Fatalf("unexpected final snapshot %+v", snapshot)
	}
}
