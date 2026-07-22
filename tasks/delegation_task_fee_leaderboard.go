package tasks

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/service"
	"time"

	log "github.com/sirupsen/logrus"
)

func StartDelegationTaskFeeLeaderboardRefresh(ctx context.Context) {
	refresh := func(ctx context.Context) {
		if err := service.RebuildDelegationTaskFeeLeaderboardSnapshots(ctx, config.GetDB(), time.Now().UTC()); err != nil {
			log.Errorf("DelegationTaskFeeLeaderboard: refresh error %v", err)
		}
	}

	refresh(ctx)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Errorf("DelegationTaskFeeLeaderboard: stop refresh due to %v", ctx.Err())
			return
		case <-ticker.C:
			refresh(ctx)
		}
	}
}
