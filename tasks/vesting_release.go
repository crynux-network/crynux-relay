package tasks

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/service"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	vestingReleaseTaskInterval = time.Hour
	vestingReleaseBatchSize    = 200
)

func StartVestingRelease(ctx context.Context) {
	ticker := time.NewTicker(vestingReleaseTaskInterval)
	defer ticker.Stop()

	run := func() {
		now := time.Now().UTC()
		if err := service.ProcessDueVestingReleases(ctx, config.GetDB(), now, vestingReleaseBatchSize); err != nil {
			log.Errorf("failed to process vesting releases: %v", err)
			return
		}
		if err := service.RefreshNodeVestingScoreStakes(ctx, config.GetDB(), now); err != nil {
			log.Errorf("failed to refresh node vesting score stakes: %v", err)
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
