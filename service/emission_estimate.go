package service

import (
	"context"
	"crynux_relay/models"
	"math/big"
	"sync"
	"time"

	"gorm.io/gorm"
)

type EmissionEstimateResult struct {
	EstimatedEmission *big.Int
	EmissionWeekStart int64
	EmissionWeekEnd   int64
	EstimateUpdatedAt int64
}

type currentEmissionEstimateSnapshot struct {
	totalTaskFee                       *big.Int
	operatorTaskFeeByNode              map[string]*big.Int
	delegationTaskFeeByNode            map[string]*big.Int
	delegationTaskFeeByUser            map[string]*big.Int
	delegationTaskFeeByUserNodeNetwork map[string]*big.Int
	nodeEmissionPoolCNX                int64
	emissionWeekStart                  time.Time
	emissionWeekEnd                    time.Time
	updatedAt                          time.Time
}

type nodeEmissionEstimateAggregateRow struct {
	NodeAddress      string
	OperatorTaskFee  string
	DelegatorTaskFee string
}

type addressEmissionEstimateAggregateRow struct {
	Address string
	TaskFee string
}

type userNodeNetworkEmissionEstimateAggregateRow struct {
	UserAddress string
	NodeAddress string
	Network     string
	TaskFee     string
}

var globalCurrentEmissionEstimateSnapshot = newCurrentEmissionEstimateSnapshot()

var currentEmissionEstimateSnapshotMutex sync.RWMutex

func newCurrentEmissionEstimateSnapshot() *currentEmissionEstimateSnapshot {
	return &currentEmissionEstimateSnapshot{
		totalTaskFee:                       big.NewInt(0),
		operatorTaskFeeByNode:              map[string]*big.Int{},
		delegationTaskFeeByNode:            map[string]*big.Int{},
		delegationTaskFeeByUser:            map[string]*big.Int{},
		delegationTaskFeeByUserNodeNetwork: map[string]*big.Int{},
	}
}

func InitCurrentEmissionEstimateSnapshot(ctx context.Context, db *gorm.DB, mainnetStartTime string) error {
	return RefreshCurrentEmissionEstimateSnapshot(ctx, db, time.Now().UTC(), mainnetStartTime)
}

func RefreshCurrentEmissionEstimateSnapshot(ctx context.Context, db *gorm.DB, now time.Time, mainnetStartTime string) error {
	weekInfo, err := GetCurrentEmissionWeekInfo(now, mainnetStartTime)
	if err != nil {
		return err
	}

	snapshot := newCurrentEmissionEstimateSnapshot()
	snapshot.nodeEmissionPoolCNX = weekInfo.NodeEmissionPoolCNX
	snapshot.emissionWeekStart = weekInfo.WeekStartDate
	snapshot.emissionWeekEnd = weekInfo.WeekEndDate
	snapshot.updatedAt = now.UTC()

	nodeRows := make([]nodeEmissionEstimateAggregateRow, 0)
	if err := db.WithContext(ctx).Model(&models.NodeEarning{}).
		Select("node_address, SUM(CAST(operator_earning AS DECIMAL(65,0))) as operator_task_fee, SUM(CAST(delegator_earning AS DECIMAL(65,0))) as delegator_task_fee").
		Where("time >= ? AND time < ?", weekInfo.WeekStartDate, weekInfo.WeekEndDate).
		Group("node_address").
		Scan(&nodeRows).Error; err != nil {
		return err
	}
	for _, row := range nodeRows {
		operatorTaskFee := parsePositiveEmissionTaskFee(row.OperatorTaskFee)
		if operatorTaskFee.Sign() > 0 {
			snapshot.operatorTaskFeeByNode[row.NodeAddress] = operatorTaskFee
			snapshot.totalTaskFee.Add(snapshot.totalTaskFee, operatorTaskFee)
		}
		delegatorTaskFee := parsePositiveEmissionTaskFee(row.DelegatorTaskFee)
		if delegatorTaskFee.Sign() > 0 {
			snapshot.delegationTaskFeeByNode[row.NodeAddress] = delegatorTaskFee
		}
	}

	userRows := make([]addressEmissionEstimateAggregateRow, 0)
	if err := db.WithContext(ctx).Model(&models.UserEarning{}).
		Select("user_address as address, SUM(CAST(earning AS DECIMAL(65,0))) as task_fee").
		Where("time >= ? AND time < ?", weekInfo.WeekStartDate, weekInfo.WeekEndDate).
		Group("user_address").
		Scan(&userRows).Error; err != nil {
		return err
	}
	for _, row := range userRows {
		taskFee := parsePositiveEmissionTaskFee(row.TaskFee)
		if taskFee.Sign() > 0 {
			snapshot.delegationTaskFeeByUser[row.Address] = taskFee
			snapshot.totalTaskFee.Add(snapshot.totalTaskFee, taskFee)
		}
	}

	userNodeNetworkRows := make([]userNodeNetworkEmissionEstimateAggregateRow, 0)
	if err := db.WithContext(ctx).Model(&models.UserStakingEarning{}).
		Select("user_address, node_address, network, SUM(CAST(earning AS DECIMAL(65,0))) as task_fee").
		Where("time >= ? AND time < ?", weekInfo.WeekStartDate, weekInfo.WeekEndDate).
		Group("user_address, node_address, network").
		Scan(&userNodeNetworkRows).Error; err != nil {
		return err
	}
	for _, row := range userNodeNetworkRows {
		taskFee := parsePositiveEmissionTaskFee(row.TaskFee)
		if taskFee.Sign() > 0 {
			snapshot.delegationTaskFeeByUserNodeNetwork[userNodeNetworkEmissionEstimateKey(row.UserAddress, row.NodeAddress, row.Network)] = taskFee
		}
	}

	currentEmissionEstimateSnapshotMutex.Lock()
	globalCurrentEmissionEstimateSnapshot = snapshot
	currentEmissionEstimateSnapshotMutex.Unlock()

	return nil
}

func GetNodeOperatorEmissionEstimate(nodeAddress string) EmissionEstimateResult {
	return estimateFromSnapshot(func(snapshot *currentEmissionEstimateSnapshot) *big.Int {
		return snapshot.operatorTaskFeeByNode[nodeAddress]
	})
}

func GetNodeDelegationEmissionEstimate(nodeAddress string) EmissionEstimateResult {
	return estimateFromSnapshot(func(snapshot *currentEmissionEstimateSnapshot) *big.Int {
		return snapshot.delegationTaskFeeByNode[nodeAddress]
	})
}

const weeklyTaskFeeMinElapsed = 24 * time.Hour
const weeklyTaskFeeMaxElapsed = 7 * 24 * time.Hour

// Scales the partial-week accumulated delegator task fee to a full-week
// equivalent by the elapsed time within the current emission week.
func GetNodeDelegationWeeklyTaskFeeEstimate(nodeAddress string) *big.Int {
	currentEmissionEstimateSnapshotMutex.RLock()
	defer currentEmissionEstimateSnapshotMutex.RUnlock()

	snapshot := globalCurrentEmissionEstimateSnapshot
	taskFee := snapshot.delegationTaskFeeByNode[nodeAddress]
	if taskFee == nil || taskFee.Sign() <= 0 {
		return big.NewInt(0)
	}

	elapsed := snapshot.updatedAt.Sub(snapshot.emissionWeekStart)
	if elapsed < weeklyTaskFeeMinElapsed {
		elapsed = weeklyTaskFeeMinElapsed
	}
	if elapsed > weeklyTaskFeeMaxElapsed {
		elapsed = weeklyTaskFeeMaxElapsed
	}

	scaled := big.NewInt(0).Mul(taskFee, big.NewInt(int64(weeklyTaskFeeMaxElapsed/time.Second)))
	return scaled.Div(scaled, big.NewInt(int64(elapsed/time.Second)))
}

func GetUserDelegationEmissionEstimate(userAddress string) EmissionEstimateResult {
	return estimateFromSnapshot(func(snapshot *currentEmissionEstimateSnapshot) *big.Int {
		return snapshot.delegationTaskFeeByUser[userAddress]
	})
}

func GetSingleDelegationEmissionEstimate(userAddress, nodeAddress, network string) EmissionEstimateResult {
	return estimateFromSnapshot(func(snapshot *currentEmissionEstimateSnapshot) *big.Int {
		return snapshot.delegationTaskFeeByUserNodeNetwork[userNodeNetworkEmissionEstimateKey(userAddress, nodeAddress, network)]
	})
}

func estimateFromSnapshot(scopeTaskFee func(snapshot *currentEmissionEstimateSnapshot) *big.Int) EmissionEstimateResult {
	currentEmissionEstimateSnapshotMutex.RLock()
	defer currentEmissionEstimateSnapshotMutex.RUnlock()

	snapshot := globalCurrentEmissionEstimateSnapshot
	result := EmissionEstimateResult{
		EstimatedEmission: big.NewInt(0),
		EmissionWeekStart: snapshot.emissionWeekStart.Unix(),
		EmissionWeekEnd:   snapshot.emissionWeekEnd.Unix(),
		EstimateUpdatedAt: snapshot.updatedAt.Unix(),
	}

	taskFee := scopeTaskFee(snapshot)
	if taskFee == nil || taskFee.Sign() == 0 || snapshot.totalTaskFee.Sign() == 0 {
		return result
	}

	emission := big.NewInt(0).Mul(taskFee, big.NewInt(snapshot.nodeEmissionPoolCNX))
	result.EstimatedEmission = emission.Div(emission, snapshot.totalTaskFee)
	return result
}

func parsePositiveEmissionTaskFee(raw string) *big.Int {
	taskFee, ok := big.NewInt(0).SetString(raw, 10)
	if !ok || taskFee.Sign() <= 0 {
		return big.NewInt(0)
	}
	return taskFee
}

func userNodeNetworkEmissionEstimateKey(userAddress, nodeAddress, network string) string {
	return userAddress + "\x00" + nodeAddress + "\x00" + network
}
