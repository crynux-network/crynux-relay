package service

import (
	"context"
	"crynux_relay/models"
	"math/big"
	"sync"
	"time"

	"gorm.io/gorm"
)

var weiPerGwei = big.NewInt(1_000_000_000)

type QueuedTaskPrioritySnapshot struct {
	AsOf                int64
	QueuedTaskCount     int64
	HighestPriorityGwei *models.BigInt
	MedianPriorityGwei  *models.BigInt
	LowestPriorityGwei  *models.BigInt
}

var (
	queuedTaskPrioritySnapshotMutex sync.RWMutex
	queuedTaskPrioritySnapshot      = QueuedTaskPrioritySnapshot{}
)

func InitQueuedTaskPrioritySnapshot(ctx context.Context, db *gorm.DB) error {
	return RefreshQueuedTaskPrioritySnapshot(ctx, db, time.Now().UTC())
}

func RefreshQueuedTaskPrioritySnapshot(ctx context.Context, db *gorm.DB, now time.Time) error {
	rangeResult, err := models.GetQueuedTaskPriorityRange(ctx, db)
	if err != nil {
		return err
	}

	snapshot := QueuedTaskPrioritySnapshot{
		AsOf:            now.UTC().Unix(),
		QueuedTaskCount: rangeResult.Count,
	}
	if rangeResult.Count > 0 {
		snapshot.HighestPriorityGwei = weiPriorityToGwei(rangeResult.Highest)
		snapshot.MedianPriorityGwei = weiPriorityToGwei(rangeResult.Median)
		snapshot.LowestPriorityGwei = weiPriorityToGwei(rangeResult.Lowest)
	}

	queuedTaskPrioritySnapshotMutex.Lock()
	queuedTaskPrioritySnapshot = snapshot
	queuedTaskPrioritySnapshotMutex.Unlock()
	return nil
}

func GetQueuedTaskPrioritySnapshot() QueuedTaskPrioritySnapshot {
	queuedTaskPrioritySnapshotMutex.RLock()
	defer queuedTaskPrioritySnapshotMutex.RUnlock()
	return copyQueuedTaskPrioritySnapshot(queuedTaskPrioritySnapshot)
}

func weiPriorityToGwei(priority *models.BigInt) *models.BigInt {
	if priority == nil {
		return nil
	}
	gwei := new(big.Int).Div(&priority.Int, weiPerGwei)
	return &models.BigInt{Int: *gwei}
}

func copyQueuedTaskPrioritySnapshot(src QueuedTaskPrioritySnapshot) QueuedTaskPrioritySnapshot {
	dst := QueuedTaskPrioritySnapshot{
		AsOf:            src.AsOf,
		QueuedTaskCount: src.QueuedTaskCount,
	}
	if src.HighestPriorityGwei != nil {
		dst.HighestPriorityGwei = &models.BigInt{Int: *new(big.Int).Set(&src.HighestPriorityGwei.Int)}
	}
	if src.MedianPriorityGwei != nil {
		dst.MedianPriorityGwei = &models.BigInt{Int: *new(big.Int).Set(&src.MedianPriorityGwei.Int)}
	}
	if src.LowestPriorityGwei != nil {
		dst.LowestPriorityGwei = &models.BigInt{Int: *new(big.Int).Set(&src.LowestPriorityGwei.Int)}
	}
	return dst
}
