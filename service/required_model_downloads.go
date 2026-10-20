package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// EnsureRequiredModelDownloads emits DownloadModel events for every configured
// required model file set whose min_vram the node meets and that the node's
// on-disk inventory does not already cover. Existing non-expired selections
// are left unchanged.
func EnsureRequiredModelDownloads(ctx context.Context, db *gorm.DB, node *models.Node) error {
	if node == nil {
		return nil
	}
	if node.Status != models.NodeStatusAvailable && node.Status != models.NodeStatusBusy {
		return nil
	}

	appCfg := config.GetConfig()
	if appCfg == nil {
		return nil
	}
	cfg := appCfg.ModelDistribution
	items := requiredModelsMatchingVRAM(cfg.RequiredModelsByVRAM, node.GPUVram)
	if len(items) == 0 {
		return nil
	}

	rows, err := models.ListNodeModelFileRows(ctx, db, node.Address)
	if err != nil {
		return err
	}
	inventories, err := nodeFileSetsByAddress(rows)
	if err != nil {
		return err
	}
	inventory := inventories[node.Address]
	if inventory == nil {
		inventory = &nodeFileSets{
			exact:  make(map[models.Hash32]struct{}),
			source: make(map[models.Hash32]struct{}),
		}
	}

	selections, err := models.GetNodeModelDownloadSelectionsByNodeAddress(ctx, db, node.Address)
	if err != nil {
		return err
	}
	nonExpiredSets := make(map[models.Hash32]struct{})
	for i := range selections {
		if selections[i].Status != models.NodeModelDownloadSelectionPending &&
			selections[i].Status != models.NodeModelDownloadSelectionCompleted {
			continue
		}
		setKey, err := models.Hash32FromBytes(selections[i].ModelFileHash)
		if err != nil {
			return err
		}
		nonExpiredSets[setKey] = struct{}{}
	}

	now := time.Now()
	for _, item := range items {
		modelFiles, exact, source, err := requiredModelFilesFromConfig(item.ModelFiles)
		if err != nil {
			return err
		}
		setKey := models.DistributionKey(exact, source)
		if modelFilesCovered(inventory.exact, inventory.source, exact, source) {
			continue
		}
		if _, ok := nonExpiredSets[setKey]; ok {
			continue
		}
		if err := emitModelDownloadForNode(
			ctx,
			db,
			exact,
			source,
			modelFiles,
			node.Address,
			item.MinVRAM,
			models.TaskType(item.TaskType),
			now,
			cfg.DownloadTimeoutSeconds,
		); err != nil {
			return err
		}
		nonExpiredSets[setKey] = struct{}{}
	}
	return nil
}

func requiredModelsMatchingVRAM(items []config.RequiredModelsByVRAMConfig, gpuVram uint64) []config.RequiredModelsByVRAMConfig {
	matched := make([]config.RequiredModelsByVRAMConfig, 0, len(items))
	for _, item := range items {
		if gpuVram >= item.MinVRAM {
			matched = append(matched, item)
		}
	}
	return matched
}

func requiredModelFilesFromConfig(files []config.RequiredModelFileEntryConfig) (entries []models.ModelFileEntry, exact, source []models.Hash32, err error) {
	entries = make([]models.ModelFileEntry, 0, len(files))
	exact = make([]models.Hash32, 0, len(files))
	source = make([]models.Hash32, 0, len(files))
	for _, file := range files {
		formatKwargs := file.FormatKwargs
		if formatKwargs == nil {
			formatKwargs = map[string]any{}
		}
		entry := models.ModelFileEntry{
			SourceType:   file.SourceType,
			Repository:   file.Repository,
			Revision:     file.Revision,
			Format:       file.Format,
			FormatKwargs: formatKwargs,
		}
		entries = append(entries, entry)
		if entry.Revision != "" {
			hash, hashErr := entry.ModelFileHash()
			if hashErr != nil {
				return nil, nil, nil, fmt.Errorf("required model file %q: %w", entry.Repository, hashErr)
			}
			exact = append(exact, hash)
			continue
		}
		hash, hashErr := entry.ModelFileSourceHash()
		if hashErr != nil {
			return nil, nil, nil, fmt.Errorf("required model file source %q: %w", entry.Repository, hashErr)
		}
		source = append(source, hash)
	}
	return entries, exact, source, nil
}

// requiredModelSetKeysFromConfig returns the DistributionKey of every
// configured required model file set.
func requiredModelSetKeysFromConfig(items []config.RequiredModelsByVRAMConfig) (map[models.Hash32]struct{}, error) {
	keys := make(map[models.Hash32]struct{}, len(items))
	for _, item := range items {
		_, exact, source, err := requiredModelFilesFromConfig(item.ModelFiles)
		if err != nil {
			return nil, err
		}
		keys[models.DistributionKey(exact, source)] = struct{}{}
	}
	return keys, nil
}
