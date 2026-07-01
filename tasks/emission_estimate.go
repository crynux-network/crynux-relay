package tasks

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/service"
	"time"

	log "github.com/sirupsen/logrus"
)

const emissionEstimateRefreshInterval = 4 * time.Hour

func StartCurrentEmissionEstimateSnapshotRefresh(ctx context.Context) {
	refresh := func(ctx context.Context) {
		if err := service.RefreshCurrentEmissionEstimateSnapshot(ctx, config.GetDB(), time.Now().UTC(), config.GetConfig().Dao.MainnetStartTime); err != nil {
			log.Errorf("CurrentEmissionEstimateSnapshot: refresh error %v", err)
		}
	}

	refresh(ctx)
	ticker := time.NewTicker(emissionEstimateRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Errorf("CurrentEmissionEstimateSnapshot: stop refresh due to %v", ctx.Err())
			return
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 10*time.Minute)
				defer cancel()
				refresh(ctx1)
			}()
		}
	}
}
