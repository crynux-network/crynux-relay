package tasks

import (
	"context"
	"time"

	"crynux_relay/config"
	"crynux_relay/service"

	log "github.com/sirupsen/logrus"
)

const historyCleanupTaskInterval = time.Hour

func StartHistoryCleanup(ctx context.Context) {
	ticker := time.NewTicker(historyCleanupTaskInterval)
	defer ticker.Stop()

	run := func() {
		conf := config.GetConfig()
		retentionDays := conf.Task.HistoryRetentionDays
		if retentionDays == 0 {
			return
		}

		cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
		db := config.GetDB()

		if err := service.CleanupTaskHistory(
			ctx,
			db,
			cutoff,
			conf.DataDir.InferenceTasks,
			conf.DataDir.SlashedTasks,
			0,
		); err != nil {
			log.Errorf("failed to cleanup task history: %v", err)
			return
		}

		if err := service.CleanupEventHistory(ctx, db, cutoff, 0); err != nil {
			log.Errorf("failed to cleanup event history: %v", err)
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
