package service

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"
	"strings"
	"time"
)

const (
	nodeHealthEventTaskTimeout = "Task Timeout"
	nodeHealthEventHealthBoost = "Health Boost"
	nodeHealthEventNodeKickout = "Node Kickout"
)

type nodeHealthMetrics struct {
	HealthBefore float64
	HealthAfter  float64
	QosBefore    float64
	QosAfter     float64
	LongTermQos  float64
}

func calculatePenaltyNodeHealthMetrics(node *models.Node) nodeHealthMetrics {
	healthBefore := getEffectiveHealth(node.HealthBase, node.HealthUpdatedAt)
	healthAfter := calculatePenalizedHealth(healthBefore)
	longTermQos := CalculateLongTermQos(node.QOSScore)

	return nodeHealthMetrics{
		HealthBefore: healthBefore,
		HealthAfter:  healthAfter,
		QosBefore:    calculateCombinedQos(longTermQos, healthBefore),
		QosAfter:     calculateCombinedQos(longTermQos, healthAfter),
		LongTermQos:  longTermQos,
	}
}

func calculateBoostNodeHealthMetrics(node *models.Node) nodeHealthMetrics {
	healthBefore := getEffectiveHealth(node.HealthBase, node.HealthUpdatedAt)
	healthAfter := calculateBoostedHealth(healthBefore)
	longTermQos := CalculateLongTermQos(node.QOSScore)

	return nodeHealthMetrics{
		HealthBefore: healthBefore,
		HealthAfter:  healthAfter,
		QosBefore:    calculateCombinedQos(longTermQos, healthBefore),
		QosAfter:     calculateCombinedQos(longTermQos, healthAfter),
		LongTermQos:  longTermQos,
	}
}

func calculateCurrentNodeHealthMetrics(node *models.Node) nodeHealthMetrics {
	health := getEffectiveHealth(node.HealthBase, node.HealthUpdatedAt)
	longTermQos := CalculateLongTermQos(node.QOSScore)
	qos := calculateCombinedQos(longTermQos, health)

	return nodeHealthMetrics{
		HealthBefore: health,
		HealthAfter:  health,
		QosBefore:    qos,
		QosAfter:     qos,
		LongTermQos:  longTermQos,
	}
}

func shouldLogHealthBoost(metrics nodeHealthMetrics) bool {
	return metrics.HealthAfter != metrics.HealthBefore
}

func logTaskTimeoutNodeHealthEvent(node *models.Node, task *models.InferenceTask, metrics nodeHealthMetrics) {
	logNodeHealthEvent(nodeHealthEventTaskTimeout, node, task, metrics)
}

func logHealthBoostNodeHealthEvent(node *models.Node, task *models.InferenceTask, metrics nodeHealthMetrics) {
	logNodeHealthEvent(nodeHealthEventHealthBoost, node, task, metrics)
}

func logNodeKickoutHealthEvent(node *models.Node, task *models.InferenceTask, metrics nodeHealthMetrics) {
	logNodeHealthEvent(nodeHealthEventNodeKickout, node, task, metrics)
}

func logNodeHealthEvent(eventName string, node *models.Node, task *models.InferenceTask, metrics nodeHealthMetrics) {
	logger := config.GetNodeHealthLogger()
	if logger == nil {
		return
	}
	logger.Infof(
		"[NodeHealth] [Node %s] [%s] [%s] task_id=%s model=%s node_card=%q gpu_vram=%dGB node_staking_score=%.4f health=%.4f->%.4f qos=%.4f->%.4f long_term_qos=%.4f",
		node.Address,
		eventName,
		taskTypeTag(task.TaskType),
		task.TaskIDCommitment,
		taskModelLabel(task),
		node.GPUName,
		node.GPUVram,
		CalculateNodeSelectingProb(*node, time.Now().UTC()).StakingScore,
		metrics.HealthBefore,
		metrics.HealthAfter,
		metrics.QosBefore,
		metrics.QosAfter,
		metrics.LongTermQos,
	)
}

func taskTypeTag(taskType models.TaskType) string {
	switch taskType {
	case models.TaskTypeLLM:
		return "LLM"
	case models.TaskTypeSD:
		return "SD"
	case models.TaskTypeSDFTLora:
		return "SDFTLora"
	default:
		return fmt.Sprintf("Unknown(%d)", taskType)
	}
}

func taskModelLabel(task *models.InferenceTask) string {
	if len(task.ModelIDs) == 0 {
		return ""
	}
	return strings.Join(task.ModelIDs, ",")
}
