package service

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"strings"
	"time"
)

func LogNodeStatusChange(node *models.Node, action string) {
	logger := config.GetNodeStatusLogger()
	if logger == nil || node == nil {
		return
	}

	qosLong, qosShort, qos := CalculateQosComponents(node.QOSScore, node.HealthBase, node.HealthUpdatedAt)
	logger.Infof(
		"[Node %s] [%s] [%s] [%dGB] [staking_score=%.4f] [qos_long=%.4f] [qos_short=%.4f] [qos=%.4f]",
		nodeActionLabel(action),
		node.Address,
		node.GPUName,
		node.GPUVram,
		CalculateNodeSelectingProb(*node, time.Now().UTC()).StakingScore,
		qosLong,
		qosShort,
		qos,
	)
}

func nodeActionLabel(action string) string {
	if action == "" {
		return "Status"
	}
	return strings.ToUpper(action[:1]) + action[1:]
}
