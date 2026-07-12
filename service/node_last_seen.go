package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Nodes poll the relay every second; the last-seen timestamp is written to the
// DB at most once per node per interval to avoid a constant stream of writes.
const nodeLastSeenWriteInterval = 60 * time.Second

var nodeLastSeenWrites sync.Map

// TouchNodeLastSeen refreshes the node's last_seen_time, throttled in memory
// so the DB write happens at most once per node per interval.
func TouchNodeLastSeen(ctx context.Context, db *gorm.DB, address string) error {
	now := time.Now()
	if lastWrite, ok := nodeLastSeenWrites.Load(address); ok {
		if now.Sub(lastWrite.(time.Time)) < nodeLastSeenWriteInterval {
			return nil
		}
	}
	nodeLastSeenWrites.Store(address, now)

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Model(&models.Node{}).
		Where("address = ?", address).
		Update("last_seen_time", sql.NullTime{Time: now, Valid: true}).Error
}
