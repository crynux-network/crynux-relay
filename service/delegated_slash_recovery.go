package service

import (
	"context"
	"crynux_relay/models"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func StartDelegatedSlashRecovery(ctx context.Context, db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		if err := recoverDelegatedSlashJobs(ctx, db); err != nil {
			log.Errorf("DelegatedSlashRecovery: failed to recover jobs: %v", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := recoverDelegatedSlashJobs(ctx, db); err != nil {
					log.Errorf("DelegatedSlashRecovery: failed to recover jobs: %v", err)
				}
			}
		}
	}()
}

func recoverDelegatedSlashJobs(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var jobs []models.DelegatedSlashJob
	if err := db.WithContext(dbCtx).
		Where("status <> ?", models.DelegatedSlashJobStatusCompleted).
		Order("id").
		Find(&jobs).Error; err != nil {
		return err
	}
	for _, job := range jobs {
		if err := sendNextDelegatedSlashBatch(dbCtx, db, job.NodeAddress, job.Network); err != nil {
			return err
		}
	}
	return nil
}
