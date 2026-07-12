package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/metrics"
	"crynux_relay/models"
	"database/sql"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	TASK_SCORE_REWARDS [3]uint64        = [3]uint64{10, 5, 2}
	nodeQoSScorePool   NodeQosScorePool = NodeQosScorePool{
		pool: make(map[string][]uint64),
	}
)

func getTaskQosScore(order int) uint64 {
	return TASK_SCORE_REWARDS[order]
}

type NodeQosScorePool struct {
	mu   sync.RWMutex
	pool map[string][]uint64
}

func getNodeTaskQosScore(node *models.Node, qos uint64) (float64, error) {
	poolSize := config.GetConfig().QoS.ScorePoolSize
	if poolSize == 0 {
		poolSize = 50
	}

	nodeQoSScorePool.mu.RLock()
	qosScorePool, ok := nodeQoSScorePool.pool[node.Address]
	nodeQoSScorePool.mu.RUnlock()
	if !ok {
		qosScorePool = make([]uint64, 0)
		if node.QOSScore > 0 {
			for i := 0; i < int(poolSize)-1; i++ {
				qosScorePool = append(qosScorePool, uint64(node.QOSScore))
			}
		}
	}
	qosScorePool = append(qosScorePool, qos)
	if len(qosScorePool) > int(poolSize) {
		qosScorePool = qosScorePool[1:]
	}

	nodeQoSScorePool.mu.Lock()
	nodeQoSScorePool.pool[node.Address] = qosScorePool
	nodeQoSScorePool.mu.Unlock()
	var sum uint64 = 0
	for _, score := range qosScorePool {
		sum += score
	}
	return float64(sum) / float64(len(qosScorePool)), nil
}

// ShouldPermanentKickout returns true if the node should be kicked out by
// either long-term QoS or qos short threshold.
func ShouldPermanentKickout(node *models.Node) bool {
	if node == nil {
		return false
	}
	cfg := config.GetConfig().QoS
	if getEffectiveHealth(node.HealthBase, node.HealthUpdatedAt) < cfg.HealthKickoutThreshold {
		return true
	}

	poolSize := cfg.ScorePoolSize
	nodeQoSScorePool.mu.RLock()
	qosScorePool, ok := nodeQoSScorePool.pool[node.Address]
	nodeQoSScorePool.mu.RUnlock()

	// Only apply long-term QoS kickout if we have enough samples.
	if !ok || uint64(len(qosScorePool)) < poolSize {
		return false
	}

	return node.QOSScore < cfg.KickoutThreshold
}

// getEffectiveHealth computes the current effective health from health fields
// using exponential decay recovery toward 1.0.
// If HealthUpdatedAt is not set, the node is considered fully healthy.
func getEffectiveHealth(healthBase float64, healthUpdatedAt sql.NullTime) float64 {
	if !healthUpdatedAt.Valid {
		return 1.0
	}
	cfg := config.GetConfig().QoS
	tau := cfg.RecoveryTauMinutes
	if tau <= 0 {
		tau = 30.0
	}

	elapsed := time.Since(healthUpdatedAt.Time).Minutes()
	if elapsed < 0 {
		elapsed = 0
	}

	// H_effective(t) = H_base + (1 - H_base) * (1 - exp(-(t - t_base) / tau))
	hEffective := healthBase + (1.0-healthBase)*(1.0-math.Exp(-elapsed/tau))
	if hEffective > 1.0 {
		hEffective = 1.0
	}
	return hEffective
}

func calculatePenalizedHealth(hEffective float64) float64 {
	cfg := config.GetConfig().QoS
	penaltyFactor := cfg.PenaltyFactor
	if hEffective >= cfg.FirstTimeoutHealthThreshold {
		penaltyFactor = cfg.FirstTimeoutPenaltyFactor
	}
	return hEffective * penaltyFactor
}

func calculateBoostedHealth(hEffective float64) float64 {
	hNew := hEffective + config.GetConfig().QoS.SuccessBoost
	if hNew > 1.0 {
		return 1.0
	}
	return hNew
}

func calculateCombinedQos(qosLong float64, health float64) float64 {
	return qosLong * health
}

// ApplyHealthPenalty is called when a task times out. It multiplies the
// current effective health by the configured penalty factor.
func ApplyHealthPenalty(ctx context.Context, db *gorm.DB, node *models.Node) error {
	hEffective := getEffectiveHealth(node.HealthBase, node.HealthUpdatedAt)
	hNew := calculatePenalizedHealth(hEffective)
	updatedAt := sql.NullTime{Time: time.Now(), Valid: true}

	if err := node.Update(ctx, db, map[string]interface{}{
		"health_base":       hNew,
		"health_updated_at": updatedAt,
	}); err != nil {
		return err
	}
	node.HealthBase = hNew
	node.HealthUpdatedAt = updatedAt
	metrics.NodeHealthPenalties.Inc()
	return nil
}

// ApplyHealthBoost is called on successful task completion. It adds the
// configured success boost to the current effective health, capped at 1.0.
func ApplyHealthBoost(ctx context.Context, db *gorm.DB, node *models.Node) error {
	hEffective := getEffectiveHealth(node.HealthBase, node.HealthUpdatedAt)
	hNew := calculateBoostedHealth(hEffective)
	updatedAt := sql.NullTime{Time: time.Now(), Valid: true}

	if err := node.Update(ctx, db, map[string]interface{}{
		"health_base":       hNew,
		"health_updated_at": updatedAt,
	}); err != nil {
		return err
	}
	node.HealthBase = hNew
	node.HealthUpdatedAt = updatedAt
	return nil
}

// CalculateQosScore returns the node's current QoS score (0 to 1),
// combining long-term performance and short-term reliability.
func CalculateQosScore(qosScore float64, healthBase float64, healthUpdatedAt sql.NullTime) float64 {
	_, _, qos := CalculateQosComponents(qosScore, healthBase, healthUpdatedAt)
	return qos
}

// CalculateLongTermQos returns normalized long-term QoS in range [0, 1].
// If persisted long-term score is not initialized, use 0.5 as default.
func CalculateLongTermQos(qosScore float64) float64 {
	qosLong := qosScore / GetMaxQosScore()
	if qosLong == 0 {
		qosLong = 0.5
	}
	return qosLong
}

// AdjustNodeQosForJoin applies a small long-term QoS recovery when an existing
// node rejoins with very low long-term QoS.
func AdjustNodeQosForJoin(node *models.Node, isNewNode bool) {
	if node == nil || isNewNode {
		return
	}

	rejoinQosLongFloor := config.GetConfig().QoS.RejoinQosLongFloor
	qosLong := CalculateLongTermQos(node.QOSScore)
	if qosLong >= rejoinQosLongFloor {
		return
	}

	node.QOSScore = rejoinQosLongFloor * GetMaxQosScore()
	// Drop the stale in-memory rolling pool so the next task score re-seeds it
	// from the floored persisted score instead of the pre-kickout low scores.
	resetNodeQosScorePool(node.Address)
}

func resetNodeQosScorePool(address string) {
	nodeQoSScorePool.mu.Lock()
	defer nodeQoSScorePool.mu.Unlock()
	delete(nodeQoSScorePool.pool, address)
}

// CalculateQosComponents returns long-term QoS, short-term QoS and combined QoS.
func CalculateQosComponents(qosScore float64, healthBase float64, healthUpdatedAt sql.NullTime) (float64, float64, float64) {
	h := getEffectiveHealth(healthBase, healthUpdatedAt)
	qosLong := CalculateLongTermQos(qosScore)
	return qosLong, h, calculateCombinedQos(qosLong, h)
}

func getNodeQosWindowSize(nodeAddress string) int {
	nodeQoSScorePool.mu.RLock()
	defer nodeQoSScorePool.mu.RUnlock()

	return len(nodeQoSScorePool.pool[nodeAddress])
}
