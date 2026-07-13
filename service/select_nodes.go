package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"time"

	"gonum.org/v1/gonum/stat/sampleuv"
	"gorm.io/gorm"
)

func filterNodesByGPU(ctx context.Context, gpuName string, gpuVram uint64, taskVersionNumbers [3]uint64) ([]models.Node, error) {
	allNodes := make([]models.Node, 0)

	offset := 0
	limit := 100

	for {
		nodes, err := func(ctx context.Context, offset, limit int) ([]models.Node, error) {
			nodes := make([]models.Node, 0)
			dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			err := config.GetDB().WithContext(dbCtx).Model(&models.Node{}).
				Preload("Models").
				Where(&models.Node{Status: models.NodeStatusAvailable, GPUName: gpuName, GPUVram: gpuVram, MajorVersion: taskVersionNumbers[0]}).
				Where("minor_version > ? or (minor_version = ? and patch_version >= ?)", taskVersionNumbers[1], taskVersionNumbers[1], taskVersionNumbers[2]).
				Order("id").
				Offset(offset).
				Limit(limit).
				Find(&nodes).Error
			if err != nil {
				return nil, err
			}
			return nodes, nil
		}(ctx, offset, limit)
		if err != nil {
			return nil, err
		}
		allNodes = append(allNodes, nodes...)
		if len(nodes) < limit {
			break
		}
		offset += limit
	}
	return allNodes, nil
}

func filterNodesByVram(ctx context.Context, minVram uint64, taskVersionNumbers [3]uint64) ([]models.Node, error) {
	allNodes := make([]models.Node, 0)

	offset := 0
	limit := 100

	for {
		nodes, err := func(ctx context.Context, offset, limit int) ([]models.Node, error) {
			nodes := make([]models.Node, 0)
			dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			err := config.GetDB().WithContext(dbCtx).Model(&models.Node{}).
				Preload("Models").
				Where(&models.Node{Status: models.NodeStatusAvailable, MajorVersion: taskVersionNumbers[0]}).
				Where("gpu_vram >= ?", minVram).
				Where("minor_version > ? or (minor_version = ? and patch_version >= ?)", taskVersionNumbers[1], taskVersionNumbers[1], taskVersionNumbers[2]).
				Order("id").
				Offset(offset).
				Limit(limit).
				Find(&nodes).Error
			if err != nil {
				return nil, err
			}
			return nodes, nil
		}(ctx, offset, limit)
		if err != nil {
			return nil, err
		}
		allNodes = append(allNodes, nodes...)
		if len(nodes) < limit {
			break
		}
		offset += limit
	}
	return allNodes, nil
}

func filterNodesByNodeNamePolicy(ctx context.Context, nodes []models.Node) ([]models.Node, error) {
	cfg := config.GetConfig()
	minimumNodeNameNumber := cfg.Task.MinimumNodeNameNumber
	nodeNameWhitelistEnabled := cfg.Task.NodeNameWhitelistEnabled
	if minimumNodeNameNumber == 0 && !nodeNameWhitelistEnabled {
		return nodes, nil
	}

	filtered := make([]models.Node, 0, len(nodes))
	for _, node := range nodes {
		nodeVersion := BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion)
		if nodeNameWhitelistEnabled {
			allowed, err := IsNodeNameWhitelisted(ctx, config.GetDB(), node.GPUName, node.GPUVram, nodeVersion)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
		}
		if minimumNodeNameNumber > 0 {
			count, err := GetNodeNameActiveCount(ctx, config.GetDB(), node.GPUName, node.GPUVram, nodeVersion)
			if err != nil {
				return nil, err
			}
			if count < minimumNodeNameNumber {
				continue
			}
		}
		filtered = append(filtered, node)
	}
	return filtered, nil
}

func matchModels(nodeModelIDs, taskModelIDs []string) int {
	nodeModelIDSet := make(map[string]struct{})
	for _, nodeModelID := range nodeModelIDs {
		nodeModelIDSet[nodeModelID] = struct{}{}
	}

	cnt := 0
	for _, taskModelID := range taskModelIDs {
		if _, ok := nodeModelIDSet[taskModelID]; ok {
			cnt += 1
		}
	}
	return cnt
}

func isSameModels(nodeModelIDs, taskModelIDs []string) bool {
	if len(nodeModelIDs) != len(taskModelIDs) {
		return false
	}
	return matchModels(nodeModelIDs, taskModelIDs) == len(nodeModelIDs)
}

func selectNodesByScore(nodes []models.Node, scores []float64, n int) []models.Node {
	w := sampleuv.NewWeighted(scores, nil)
	if n > len(nodes) {
		n = len(nodes)
	}
	res := make([]models.Node, n)
	for i := 0; i < n; i++ {
		idx, ok := w.Take()
		if ok {
			res[i] = nodes[idx]
		} else {
			res[i] = nodes[i]
		}
	}
	return res
}

func selectNodesForDownloadTask(ctx context.Context, task *models.InferenceTask, modelID string, n int) ([]models.Node, error) {
	var nodes []models.Node
	var err error
	taskVersionNumbers := task.VersionNumbers()
	if len(task.RequiredGPU) > 0 {
		nodes, err = filterNodesByGPU(ctx, task.RequiredGPU, task.RequiredGPUVRAM, taskVersionNumbers)
		if err != nil {
			return nil, err
		}
	} else {
		nodes, err = filterNodesByVram(ctx, task.MinVRAM, taskVersionNumbers)
		if err != nil {
			return nil, err
		}
	}
	nodes, err = filterNodesByNodeNamePolicy(ctx, nodes)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	var validNodes []models.Node
	for _, node := range nodes {
		valid := true
		for _, model := range node.Models {
			if model.ModelID == modelID {
				valid = false
				break
			}
		}
		if valid {
			validNodes = append(validNodes, node)
		}
	}
	if len(validNodes) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	scores := make([]float64, len(validNodes))
	for i, node := range validNodes {
		scores[i] = CalculateNodeSelectingProb(node, now).ProbWeight
	}

	res := selectNodesByScore(validNodes, scores, n)
	return res, nil
}

func countAvailableNodesWithModelID(ctx context.Context, db *gorm.DB, modelID string) (int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int64
	err := db.WithContext(dbCtx).
		Model(&models.NodeModel{}).
		Joins("INNER JOIN nodes on nodes.address = node_models.node_address and nodes.status = ?", models.NodeStatusAvailable).
		Where(&models.NodeModel{ModelID: modelID}).
		Count(&count).
		Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
