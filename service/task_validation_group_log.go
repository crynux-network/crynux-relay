package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"fmt"
	"strings"
)

type validationGroupNodeMetrics struct {
	Address               string
	GPUName               string
	GPUVram               uint64
	LongTermBefore        float64
	LongTermAfter         float64
	LongTermScoreBefore   float64
	LongTermScoreAfter    float64
	QosBefore             float64
	QosAfter              float64
	WindowSize            int
	StatusBefore          models.NodeStatus
	StatusAfter           models.NodeStatus
	SlashTriggered        bool
	KickoutCheckTriggered bool
}

func collectValidationGroupStatusLabels(tasks []*models.InferenceTask) []string {
	labels := make([]string, 0, len(tasks))
	for _, task := range tasks {
		labels = append(labels, validationGroupStatusLabel(task))
	}
	return labels
}

func validationGroupStatusLabel(task *models.InferenceTask) string {
	switch task.Status {
	case models.TaskScoreReady:
		return "Success"
	case models.TaskErrorReported:
		return "ErrorReported"
	case models.TaskEndAborted:
		switch task.AbortReason {
		case models.TaskAbortTimeout:
			return "AbortedTimeout"
		case models.TaskAbortModelDownloadFailed:
			return "AbortedModelDownloadFailed"
		case models.TaskAbortIncorrectResult:
			return "AbortedIncorrectResult"
		case models.TaskAbortTaskFeeTooLow:
			return "AbortedTaskFeeTooLow"
		case models.TaskAbortGroupTimeout:
			return "AbortedGroupTimeout"
		case models.TaskAbortErrorReported:
			return "AbortedErrorReported"
		default:
			return "Aborted"
		}
	default:
		return fmt.Sprintf("Status(%d)", task.Status)
	}
}

func collectValidationGroupNodeMetricsBefore(ctx context.Context, tasks []*models.InferenceTask) (map[string]validationGroupNodeMetrics, []string, error) {
	metricsByNode := make(map[string]validationGroupNodeMetrics)
	orderedNodeAddresses := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if len(task.SelectedNode) == 0 {
			continue
		}
		if _, ok := metricsByNode[task.SelectedNode]; ok {
			continue
		}

		node, err := models.GetNodeByAddress(ctx, config.GetDB(), task.SelectedNode)
		if err != nil {
			return nil, nil, err
		}

		metricsByNode[task.SelectedNode] = validationGroupNodeMetrics{
			Address:             task.SelectedNode,
			GPUName:             node.GPUName,
			GPUVram:             node.GPUVram,
			LongTermBefore:      CalculateLongTermQos(node.QOSScore),
			LongTermScoreBefore: node.QOSScore,
			QosBefore:           CalculateQosScore(node.QOSScore, node.HealthBase, node.HealthUpdatedAt),
			StatusBefore:        node.Status,
		}
		orderedNodeAddresses = append(orderedNodeAddresses, task.SelectedNode)
	}
	return metricsByNode, orderedNodeAddresses, nil
}

func markValidationGroupKickoutCheckNodes(tasks []*models.InferenceTask, nextStatusMap map[string]models.TaskStatus, metricsByNode map[string]validationGroupNodeMetrics) {
	for _, task := range tasks {
		if len(task.SelectedNode) == 0 {
			continue
		}

		nextStatus := nextStatusMap[task.TaskIDCommitment]
		if nextStatus != models.TaskGroupValidated &&
			nextStatus != models.TaskEndGroupRefund &&
			(nextStatus != models.TaskEndAborted || task.Status == models.TaskEndAborted) {
			continue
		}

		metric, ok := metricsByNode[task.SelectedNode]
		if !ok {
			continue
		}
		metric.KickoutCheckTriggered = true
		metricsByNode[task.SelectedNode] = metric
	}
}

func markValidationGroupSlashedNodes(tasks []*models.InferenceTask, nextStatusMap map[string]models.TaskStatus, metricsByNode map[string]validationGroupNodeMetrics) {
	for _, task := range tasks {
		if len(task.SelectedNode) == 0 {
			continue
		}
		if nextStatusMap[task.TaskIDCommitment] != models.TaskEndInvalidated {
			continue
		}

		metric, ok := metricsByNode[task.SelectedNode]
		if !ok {
			continue
		}
		metric.SlashTriggered = true
		metricsByNode[task.SelectedNode] = metric
	}
}

func collectValidationGroupNodeMetricsAfter(ctx context.Context, before map[string]validationGroupNodeMetrics, orderedNodeAddresses []string) ([]validationGroupNodeMetrics, error) {
	metrics := make([]validationGroupNodeMetrics, 0, len(orderedNodeAddresses))
	for _, address := range orderedNodeAddresses {
		node, err := models.GetNodeByAddress(ctx, config.GetDB(), address)
		if err != nil {
			return nil, err
		}

		entry := before[address]
		entry.LongTermAfter = CalculateLongTermQos(node.QOSScore)
		entry.LongTermScoreAfter = node.QOSScore
		entry.QosAfter = CalculateQosScore(node.QOSScore, node.HealthBase, node.HealthUpdatedAt)
		entry.WindowSize = getNodeQosWindowSize(address)
		entry.StatusAfter = node.Status
		entry.GPUName = node.GPUName
		entry.GPUVram = node.GPUVram
		metrics = append(metrics, entry)
	}
	return metrics, nil
}

func logValidationGroupEvent(taskID string, taskType models.TaskType, statuses []string, nodeMetrics []validationGroupNodeMetrics) {
	logger := config.GetTaskValidationGroupLogger()
	if logger == nil {
		return
	}

	logger.Infof(
		"[TaskValidationGroup] [%s] task_id=%s statuses=%s qos_long=%s qos=%s",
		taskTypeTag(taskType),
		taskID,
		formatValidationGroupStatuses(statuses),
		formatValidationGroupLongTermUpdates(nodeMetrics),
		formatValidationGroupQosUpdates(nodeMetrics),
	)

	slashedNodes := collectValidationGroupSlashedNodes(nodeMetrics)
	if len(slashedNodes) > 0 {
		for _, node := range slashedNodes {
			logger.Infof(
				"[TaskValidationGroup] [%s] [Node Slash] task_id=%s node=%s card=%q vram=%dGB",
				taskTypeTag(taskType),
				taskID,
				node.Address,
				node.GPUName,
				node.GPUVram,
			)
		}
	}

	kickedOutNodes := collectValidationGroupKickedOutNodes(nodeMetrics)
	if len(kickedOutNodes) == 0 {
		return
	}

	for _, node := range kickedOutNodes {
		logger.Infof(
			"[TaskValidationGroup] [%s] [Node Kickout] task_id=%s node=%s card=%q vram=%dGB",
			taskTypeTag(taskType),
			taskID,
			node.Address,
			node.GPUName,
			node.GPUVram,
		)
	}
}

func formatValidationGroupStatuses(statuses []string) string {
	return "[" + strings.Join(statuses, ", ") + "]"
}

func formatValidationGroupLongTermUpdates(nodeMetrics []validationGroupNodeMetrics) string {
	if len(nodeMetrics) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(nodeMetrics))
	for _, metric := range nodeMetrics {
		parts = append(parts, fmt.Sprintf(
			"%s %.4f->%.4f",
			metric.Address,
			metric.LongTermBefore,
			metric.LongTermAfter,
		))
	}
	return "[QoS-long: " + strings.Join(parts, ", ") + "]"
}

func formatValidationGroupQosUpdates(nodeMetrics []validationGroupNodeMetrics) string {
	if len(nodeMetrics) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(nodeMetrics))
	for _, metric := range nodeMetrics {
		parts = append(parts, fmt.Sprintf(
			"%s %.4f->%.4f",
			metric.Address,
			metric.QosBefore,
			metric.QosAfter,
		))
	}
	return "[QoS: " + strings.Join(parts, ", ") + "]"
}

func collectValidationGroupSlashedNodes(nodeMetrics []validationGroupNodeMetrics) []validationGroupNodeMetrics {
	slashedNodes := make([]validationGroupNodeMetrics, 0)
	for _, metric := range nodeMetrics {
		if !metric.SlashTriggered {
			continue
		}
		if metric.StatusAfter != models.NodeStatusQuit {
			continue
		}
		slashedNodes = append(slashedNodes, metric)
	}
	return slashedNodes
}

func collectValidationGroupKickedOutNodes(nodeMetrics []validationGroupNodeMetrics) []validationGroupNodeMetrics {
	cfg := config.GetConfig().QoS
	kickedOutNodes := make([]validationGroupNodeMetrics, 0)
	for _, metric := range nodeMetrics {
		if !metric.KickoutCheckTriggered {
			continue
		}
		if metric.StatusAfter != models.NodeStatusQuit {
			continue
		}
		if uint64(metric.WindowSize) < cfg.ScorePoolSize {
			continue
		}
		if metric.LongTermScoreAfter >= cfg.KickoutThreshold {
			continue
		}
		kickedOutNodes = append(kickedOutNodes, metric)
	}
	return kickedOutNodes
}
