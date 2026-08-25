package tasks

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/service"
	"time"

	log "github.com/sirupsen/logrus"
)

func StartQueuedTaskPrioritySnapshotRefresh(ctx context.Context) {
	intervalSeconds := config.GetConfig().TaskPricing.QueuedTaskPrioritySnapshotIntervalSeconds
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Errorf("QueuedTaskPrioritySnapshot: stop refresh due to %v", ctx.Err())
			return
		case <-ticker.C:
			func() {
				refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				if err := service.RefreshQueuedTaskPrioritySnapshot(refreshCtx, config.GetDB(), time.Now().UTC()); err != nil {
					log.Errorf("QueuedTaskPrioritySnapshot: refresh error %v", err)
				}
			}()
		}
	}
}
