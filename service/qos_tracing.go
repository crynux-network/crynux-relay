package service

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"sync"
	"time"
)

const (
	QosTraceEventValidationGroupRank1         = "validation_group_rank_1"
	QosTraceEventValidationGroupRank2         = "validation_group_rank_2"
	QosTraceEventValidationGroupRank3         = "validation_group_rank_3"
	QosTraceEventValidationGroupAborted       = "validation_group_aborted"
	QosTraceEventTaskTimeoutPenalty           = "task_timeout_penalty"
	QosTraceEventTaskResultUploadSuccessBoost = "task_result_upload_success_boost"
	QosTraceEventValidationGroupMatchedBoost  = "validation_group_matched_boost"
	QosTraceEventNodeJoinHealthReset          = "node_join_health_reset"
	QosTraceEventNodeRejoinQosFloor           = "node_rejoin_qos_floor"
)

type NodeQosTraceValues struct {
	QosLong  float64
	QosShort float64
	Qos      float64
}

type NodeQosTraceEvent struct {
	Timestamp        int64   `json:"timestamp"`
	NodeAddress      string  `json:"node_address"`
	TaskIDCommitment string  `json:"task_id_commitment,omitempty"`
	EventType        string  `json:"event_type"`
	TaskQosScore     *uint64 `json:"task_qos_score,omitempty"`
	ValidationRank   *uint64 `json:"validation_rank,omitempty"`
	QosLongBefore    float64 `json:"qos_long_before"`
	QosLongAfter     float64 `json:"qos_long_after"`
	QosShortBefore   float64 `json:"qos_short_before"`
	QosShortAfter    float64 `json:"qos_short_after"`
	QosBefore        float64 `json:"qos_before"`
	QosAfter         float64 `json:"qos_after"`
}

type NodeQosTraceInput struct {
	NodeAddress      string
	TaskIDCommitment string
	EventType        string
	TaskQosScore     *uint64
	ValidationRank   *uint64
	Before           NodeQosTraceValues
	After            NodeQosTraceValues
}

type nodeQosTraceStore struct {
	mu     sync.RWMutex
	events map[string][]NodeQosTraceEvent
}

var qosTraceStore = nodeQosTraceStore{
	events: make(map[string][]NodeQosTraceEvent),
}

func CaptureNodeQosTraceValues(node *models.Node) NodeQosTraceValues {
	if config.GetConfig() == nil {
		qosLong := CalculateLongTermQos(node.QOSScore)
		qosShort := 1.0
		if node.HealthUpdatedAt.Valid {
			qosShort = node.HealthBase
		}
		return NodeQosTraceValues{
			QosLong:  qosLong,
			QosShort: qosShort,
			Qos:      calculateCombinedQos(qosLong, qosShort),
		}
	}
	qosLong, qosShort, qos := CalculateQosComponents(node.QOSScore, node.HealthBase, node.HealthUpdatedAt)
	return NodeQosTraceValues{
		QosLong:  qosLong,
		QosShort: qosShort,
		Qos:      qos,
	}
}

func RecordNodeQosTrace(input NodeQosTraceInput) {
	if config.GetConfig() == nil || config.GetConfig().QoS.TracingMaxTaskEvents == 0 {
		return
	}
	if !qosTraceChanged(input.Before, input.After) {
		return
	}

	event := NodeQosTraceEvent{
		Timestamp:        time.Now().Unix(),
		NodeAddress:      input.NodeAddress,
		TaskIDCommitment: input.TaskIDCommitment,
		EventType:        input.EventType,
		TaskQosScore:     input.TaskQosScore,
		ValidationRank:   input.ValidationRank,
		QosLongBefore:    input.Before.QosLong,
		QosLongAfter:     input.After.QosLong,
		QosShortBefore:   input.Before.QosShort,
		QosShortAfter:    input.After.QosShort,
		QosBefore:        input.Before.Qos,
		QosAfter:         input.After.Qos,
	}

	qosTraceStore.mu.Lock()
	defer qosTraceStore.mu.Unlock()

	events := append(qosTraceStore.events[input.NodeAddress], event)
	maxEvents := int(config.GetConfig().QoS.TracingMaxTaskEvents)
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}
	qosTraceStore.events[input.NodeAddress] = events
}

func ListNodeQosTraceEvents(nodeAddress string) []NodeQosTraceEvent {
	qosTraceStore.mu.RLock()
	defer qosTraceStore.mu.RUnlock()

	events := qosTraceStore.events[nodeAddress]
	result := make([]NodeQosTraceEvent, len(events))
	copy(result, events)
	return result
}

func GetNodeQosTraceMaxTaskEvents() uint64 {
	return config.GetConfig().QoS.TracingMaxTaskEvents
}

func BuildValidationGroupQosTraceMetadata(task *models.InferenceTask) (string, *uint64) {
	if !task.QOSScore.Valid {
		return "", nil
	}
	switch uint64(task.QOSScore.Int64) {
	case TASK_SCORE_REWARDS[0]:
		return QosTraceEventValidationGroupRank1, uint64Ptr(1)
	case TASK_SCORE_REWARDS[1]:
		return QosTraceEventValidationGroupRank2, uint64Ptr(2)
	case TASK_SCORE_REWARDS[2]:
		return QosTraceEventValidationGroupRank3, uint64Ptr(3)
	case 0:
		return QosTraceEventValidationGroupAborted, nil
	default:
		return "", nil
	}
}

func qosTraceChanged(before NodeQosTraceValues, after NodeQosTraceValues) bool {
	return before.QosLong != after.QosLong || before.QosShort != after.QosShort || before.Qos != after.Qos
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}
