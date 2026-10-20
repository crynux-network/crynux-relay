package service

import (
	"context"
	"crynux_relay/models"
	"errors"

	"gorm.io/gorm"
)

var ErrNodeCapabilitiesIllegalStatus = errors.New("node capabilities cannot be updated after quit")

type NodeCapabilities struct {
	GPUName      string
	GPUVram      uint64
	MajorVersion uint64
	MinorVersion uint64
	PatchVersion uint64
	ModelFiles   []models.ModelFile
}

type nodeNameTuple struct {
	GPUName string
	GPUVram uint64
	Version string
}

func nodeNameTupleFromNode(node *models.Node) nodeNameTuple {
	return nodeNameTuple{
		GPUName: node.GPUName,
		GPUVram: node.GPUVram,
		Version: BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion),
	}
}

func SyncNodeCapabilities(ctx context.Context, db *gorm.DB, address string, capabilities NodeCapabilities) error {
	capabilities.GPUName = models.NormalizeGPUName(capabilities.GPUName)

	var oldTuple, newTuple nodeNameTuple
	var transferNodeNameCount bool
	var refreshStakeableSnapshot bool

	err := ExecuteNodeStateUpdate(ctx, db, []string{address}, func() error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			node, err := models.GetNodeByAddress(ctx, tx, address)
			if err != nil {
				return err
			}
			if node.Status == models.NodeStatusQuit {
				return ErrNodeCapabilitiesIllegalStatus
			}

			oldNode := *node
			oldTuple = nodeNameTupleFromNode(&oldNode)
			node.GPUName = capabilities.GPUName
			node.GPUVram = capabilities.GPUVram
			node.MajorVersion = capabilities.MajorVersion
			node.MinorVersion = capabilities.MinorVersion
			node.PatchVersion = capabilities.PatchVersion
			newTuple = nodeNameTupleFromNode(node)

			tupleChanged := oldTuple != newTuple
			transferNodeNameCount = tupleChanged && IsNodeStatusActiveForNodeNameCount(node.Status)
			refreshStakeableSnapshot = node.DelegatorShare > 0

			if transferNodeNameCount {
				if err := DecrementNodeNameCountTx(ctx, tx, &oldNode); err != nil {
					return err
				}
				if err := IncrementNodeNameCountTx(ctx, tx, node); err != nil {
					return err
				}
			}

			if err := tx.WithContext(ctx).Model(&models.Node{}).
				Where("address = ?", address).
				Updates(map[string]interface{}{
					"gpu_name":      node.GPUName,
					"gpu_vram":      node.GPUVram,
					"major_version": node.MajorVersion,
					"minor_version": node.MinorVersion,
					"patch_version": node.PatchVersion,
				}).Error; err != nil {
				return err
			}

			if err := replaceNodeModelFiles(ctx, tx, address, capabilities.ModelFiles); err != nil {
				return err
			}

			var networkNodeData models.NetworkNodeData
			if err := tx.WithContext(ctx).
				Where("address = ?", address).
				First(&networkNodeData).Error; err != nil {
				return err
			}
			if err := tx.WithContext(ctx).Model(&networkNodeData).
				Updates(map[string]interface{}{
					"card_model": newTuple.GPUName,
					"v_ram":      int(newTuple.GPUVram),
				}).Error; err != nil {
				return err
			}
			return nil
		})
	})
	if err != nil {
		return err
	}

	if transferNodeNameCount {
		ApplyNodeNameCountDeltaToCache(oldTuple.GPUName, oldTuple.GPUVram, oldTuple.Version, -1)
		ApplyNodeNameCountDeltaToCache(newTuple.GPUName, newTuple.GPUVram, newTuple.Version, 1)
	}
	if refreshStakeableSnapshot {
		if err := RefreshDelegatedStakingNodeListSnapshot(ctx, db, address); err != nil {
			return err
		}
	}
	node, err := models.GetNodeByAddress(ctx, db, address)
	if err != nil {
		return err
	}
	return EnsureRequiredModelDownloads(ctx, db, node)
}
