package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
)

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
