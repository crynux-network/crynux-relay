package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const delegatedStakingNodeSnapshotBatchSize = 100
const delegatedStakingAPRObservationMonths = 12

type estimatedNextDelegationAPRAmount struct {
	amount *big.Int
}

type delegationEmissionGrantRow struct {
	NodeAddress   string
	TotalEmission string
}

type delegationAPREarningRow struct {
	NodeAddress           string
	TotalDelegatorEarning string
}

type delegationAPRStakingRow struct {
	NodeAddress           string
	TotalDelegatorStaking string
	ObservationDays       uint32
}

type delegationAPRInput struct {
	TotalDelegatorEarning  *big.Int
	TotalDelegatorEmission *big.Int
	TotalDelegatorStaking  *big.Int
	ObservationDays        uint32
}

type delegationAPRProjectionNode struct {
	Address          string
	ScoreStakeAmount *big.Int
	QOSScore         float64
	ProbWeight       float64
}

type delegationAPRProjectionContext struct {
	Nodes       []delegationAPRProjectionNode
	TotalWeight float64
	MaxStaking  *big.Int
}

type delegatedStakingNodeListEstimateRow struct {
	NodeAddress string
}

func BuildDelegatedStakingNodeStatusGroup(status models.NodeStatus) (string, uint8) {
	if status == models.NodeStatusQuit {
		return models.DelegatedStakingNodeStatusGroupStopped, 1
	}
	return models.DelegatedStakingNodeStatusGroupRunning, 0
}

func buildNodeVersion(major, minor, patch uint64) string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func BuildDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, node models.Node, now time.Time, mainnetStartTime string) (*models.DelegatedStakingNodeListSnapshot, error) {
	return buildDelegatedStakingNodeListSnapshot(ctx, db, node, now, mainnetStartTime, nil, nil)
}

func buildDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, node models.Node, now time.Time, mainnetStartTime string, aprInput *delegationAPRInput, projectionContext *delegationAPRProjectionContext) (*models.DelegatedStakingNodeListSnapshot, error) {
	if node.DelegatorShare == 0 {
		return nil, nil
	}

	emissionChartRange, err := BuildEmissionChartRange(now, mainnetStartTime, 4)
	if err != nil {
		return nil, err
	}
	operatorEmission, err := getNodeOperatorEmission(ctx, db, node.Address, emissionChartRange.RangeStart, emissionChartRange.RangeEnd)
	if err != nil {
		return nil, err
	}
	delegatorEmission, err := getNodeDelegationEmission(ctx, db, node.Address, emissionChartRange.RangeStart, emissionChartRange.RangeEnd)
	if err != nil {
		return nil, err
	}

	delegatorStaking := GetNodeTotalStakeAmount(node.Address, node.Network)
	totalStaking := GetNodeScoreStakeAmount(node, now)
	operatorStaking := big.NewInt(0).Sub(totalStaking, delegatorStaking)
	delegatorsNum := GetDelegatorCountOfNode(node.Address, node.Network)
	selectingProb := CalculateNodeSelectingProb(node, now)
	operatorEmissionEstimate := GetNodeOperatorEmissionEstimate(node.Address)
	delegatorEmissionEstimate := GetNodeDelegationEmissionEstimate(node.Address)
	statusGroup, statusRank := BuildDelegatedStakingNodeStatusGroup(node.Status)
	var delegationAPR12m float64
	var aprObservationDays uint32
	if aprInput == nil {
		start, end, err := buildDelegationAPRRange(now, configuredDelegationAPRStartTime())
		if err != nil {
			return nil, err
		}
		aprInput, err = loadNodeDelegationAPRInput(ctx, db, node.Address, start, end)
		if err != nil {
			return nil, err
		}
		delegationAPR12m, aprObservationDays = calculateDelegationAPR(aprInput)
	} else {
		delegationAPR12m, aprObservationDays = calculateDelegationAPR(aprInput)
	}
	estimatedNextAPRs := calculateEstimatedNextDelegationAPRs(node.Address, delegatorStaking, aprInput, projectionContext)

	return &models.DelegatedStakingNodeListSnapshot{
		NodeAddress:                        node.Address,
		Network:                            node.Network,
		Status:                             node.Status,
		StatusGroup:                        statusGroup,
		StatusRank:                         statusRank,
		GPUName:                            node.GPUName,
		GPUVram:                            node.GPUVram,
		Version:                            buildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion),
		OperatorEmission4w:                 models.BigInt{Int: *operatorEmission},
		DelegatorEmission4w:                models.BigInt{Int: *delegatorEmission},
		OperatorStaking:                    models.BigInt{Int: *operatorStaking},
		DelegatorStaking:                   models.BigInt{Int: *delegatorStaking},
		TotalStaking:                       models.BigInt{Int: *totalStaking},
		DelegatorsNum:                      uint64(delegatorsNum),
		ProbWeight:                         selectingProb.ProbWeight,
		QOS:                                selectingProb.QOSScore,
		EstimatedUpcomingOperatorEmission:  models.BigInt{Int: *operatorEmissionEstimate.EstimatedEmission},
		EstimatedUpcomingDelegatorEmission: models.BigInt{Int: *delegatorEmissionEstimate.EstimatedEmission},
		DelegationApr12m:                   delegationAPR12m,
		EstimatedNext10kDelegationApr:      estimatedNextAPRs[0],
		EstimatedNext100kDelegationApr:     estimatedNextAPRs[1],
		EstimatedNext1mDelegationApr:       estimatedNextAPRs[2],
		AprObservationDays:                 aprObservationDays,
		DelegationAprUpdatedAt:             now.UTC(),
	}, nil
}

func CalculateNodeDelegationAPR12m(ctx context.Context, db *gorm.DB, nodeAddress string, now time.Time) (float64, uint32, error) {
	start, end, err := buildDelegationAPRRange(now, configuredDelegationAPRStartTime())
	if err != nil {
		return 0, 0, err
	}

	input, err := loadNodeDelegationAPRInput(ctx, db, nodeAddress, start, end)
	if err != nil {
		return 0, 0, err
	}
	apr, observationDays := calculateDelegationAPR(input)
	return apr, observationDays, nil
}

func configuredDelegationAPRStartTime() string {
	appConfig := config.GetConfig()
	if appConfig == nil {
		return ""
	}
	return appConfig.Dao.AprStartTime
}

func buildDelegationAPRRange(now time.Time, aprStartTime string) (time.Time, time.Time, error) {
	end := now.UTC()
	start := end.AddDate(0, -delegatedStakingAPRObservationMonths, 0)
	rawAprStartTime := strings.TrimSpace(aprStartTime)
	if rawAprStartTime == "" {
		return start, end, nil
	}

	parsedAprStartTime, err := time.Parse(time.RFC3339, rawAprStartTime)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("dao.apr_start_time must be RFC3339 format: %w", err)
	}
	aprStartUTC := parsedAprStartTime.UTC()
	aprStartDate := time.Date(aprStartUTC.Year(), aprStartUTC.Month(), aprStartUTC.Day(), 0, 0, 0, 0, time.UTC)
	if aprStartDate.After(start) {
		start = aprStartDate
	}
	return start, end, nil
}

func loadNodeDelegationAPRInput(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) (*delegationAPRInput, error) {
	earnings, err := models.GetNodeEarnings(ctx, db, nodeAddress, start, end)
	if err != nil {
		return nil, err
	}
	input := newDelegationAPRInput()
	for _, earning := range earnings {
		input.TotalDelegatorEarning.Add(input.TotalDelegatorEarning, &earning.DelegatorEarning.Int)
	}
	totalDelegatorEmission, err := getNodeDelegationEmission(ctx, db, nodeAddress, start, end)
	if err != nil {
		return nil, err
	}
	input.TotalDelegatorEmission.Add(input.TotalDelegatorEmission, totalDelegatorEmission)

	stakings, err := models.GetNodeStakings(ctx, db, nodeAddress, start, end)
	if err != nil {
		return nil, err
	}
	for _, staking := range stakings {
		input.TotalDelegatorStaking.Add(input.TotalDelegatorStaking, &staking.DelegatorStaking.Int)
	}
	input.ObservationDays = uint32(len(stakings))
	return input, nil
}

func calculateDelegationAPR(input *delegationAPRInput) (float64, uint32) {
	if input.TotalDelegatorStaking.Sign() == 0 {
		return 0, input.ObservationDays
	}

	totalDelegatorEarning := big.NewInt(0).Add(input.TotalDelegatorEarning, input.TotalDelegatorEmission)
	apr := new(big.Rat).SetInt(totalDelegatorEarning)
	apr.Mul(apr, big.NewRat(365, 1))
	apr.Quo(apr, new(big.Rat).SetInt(input.TotalDelegatorStaking))
	aprFloat, _ := apr.Float64()
	return aprFloat, input.ObservationDays
}

func estimatedNextDelegationAPRAmounts() []estimatedNextDelegationAPRAmount {
	return []estimatedNextDelegationAPRAmount{
		{amount: utils.EtherToWei(big.NewInt(10000))},
		{amount: utils.EtherToWei(big.NewInt(100000))},
		{amount: utils.EtherToWei(big.NewInt(1000000))},
	}
}

func calculateEstimatedNextDelegationAPRs(nodeAddress string, currentDelegatorStaking *big.Int, aprInput *delegationAPRInput, projectionContext *delegationAPRProjectionContext) [3]float64 {
	var res [3]float64
	if aprInput == nil || projectionContext == nil {
		return res
	}

	amounts := estimatedNextDelegationAPRAmounts()
	for i, amount := range amounts {
		currentShare, projectedShare := projectionContext.projectedNodeWeightShare(nodeAddress, amount.amount)
		res[i] = calculateEstimatedNextDelegationAPR(aprInput, currentDelegatorStaking, amount.amount, currentShare, projectedShare)
	}
	return res
}

func calculateEstimatedNextDelegationAPR(aprInput *delegationAPRInput, currentDelegatorStaking, newDelegationAmount *big.Int, currentNodeWeightShare, projectedNodeWeightShare float64) float64 {
	if aprInput == nil ||
		aprInput.ObservationDays == 0 ||
		newDelegationAmount == nil ||
		newDelegationAmount.Sign() <= 0 ||
		currentNodeWeightShare == 0 ||
		projectedNodeWeightShare == 0 {
		return 0
	}

	totalDelegatorIncome := big.NewInt(0).Add(aprInput.TotalDelegatorEarning, aprInput.TotalDelegatorEmission)
	if totalDelegatorIncome.Sign() == 0 {
		return 0
	}

	poolAfterDelegation := big.NewInt(0)
	if currentDelegatorStaking != nil {
		poolAfterDelegation.Set(currentDelegatorStaking)
	}
	poolAfterDelegation.Add(poolAfterDelegation, newDelegationAmount)
	if poolAfterDelegation.Sign() == 0 {
		return 0
	}

	apr := new(big.Rat).SetInt(totalDelegatorIncome)
	apr.Mul(apr, big.NewRat(365, 1))
	apr.Quo(apr, big.NewRat(int64(aprInput.ObservationDays), 1))
	apr.Quo(apr, new(big.Rat).SetInt(poolAfterDelegation))
	aprFloat, _ := apr.Float64()
	return aprFloat * projectedNodeWeightShare / currentNodeWeightShare
}

func newDelegationAPRProjectionContext(nodes []models.Node, now time.Time) *delegationAPRProjectionContext {
	projectionNodes := make([]delegationAPRProjectionNode, 0, len(nodes))
	maxStaking := big.NewInt(0)
	scoreStakeByAddress := make(map[string]*big.Int, len(nodes))
	for _, node := range nodes {
		scoreStake := GetNodeScoreStakeAmount(node, now)
		scoreStakeByAddress[node.Address] = scoreStake
		if scoreStake.Cmp(maxStaking) > 0 {
			maxStaking = big.NewInt(0).Set(scoreStake)
		}
	}

	totalWeight := 0.0
	for _, node := range nodes {
		scoreStake := scoreStakeByAddress[node.Address]
		qosScore := CalculateQosScore(node.QOSScore, node.HealthBase, node.HealthUpdatedAt)
		_, _, probWeight := CalculateSelectingProb(scoreStake, maxStaking, qosScore)
		totalWeight += probWeight
		projectionNodes = append(projectionNodes, delegationAPRProjectionNode{
			Address:          node.Address,
			ScoreStakeAmount: big.NewInt(0).Set(scoreStake),
			QOSScore:         qosScore,
			ProbWeight:       probWeight,
		})
	}

	return &delegationAPRProjectionContext{
		Nodes:       projectionNodes,
		TotalWeight: totalWeight,
		MaxStaking:  maxStaking,
	}
}

func loadDelegationAPRProjectionContext(ctx context.Context, db *gorm.DB, now time.Time) (*delegationAPRProjectionContext, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodes []models.Node
	if err := db.WithContext(dbCtx).Model(&models.Node{}).Where("status != ?", models.NodeStatusQuit).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return newDelegationAPRProjectionContext(nodes, now), nil
}

func (c *delegationAPRProjectionContext) projectedNodeWeightShare(nodeAddress string, addedStake *big.Int) (float64, float64) {
	if c == nil || c.TotalWeight == 0 || addedStake == nil || addedStake.Sign() <= 0 {
		return 0, 0
	}

	projectedMaxStaking := big.NewInt(0).Set(c.MaxStaking)
	for _, node := range c.Nodes {
		if node.Address != nodeAddress {
			continue
		}
		projectedStake := big.NewInt(0).Add(node.ScoreStakeAmount, addedStake)
		if projectedStake.Cmp(projectedMaxStaking) > 0 {
			projectedMaxStaking = projectedStake
		}
		break
	}

	currentNodeWeight := 0.0
	projectedNodeWeight := 0.0
	projectedTotalWeight := 0.0
	for _, node := range c.Nodes {
		projectedStake := node.ScoreStakeAmount
		if node.Address == nodeAddress {
			currentNodeWeight = node.ProbWeight
			projectedStake = big.NewInt(0).Add(node.ScoreStakeAmount, addedStake)
		}
		_, _, projectedWeight := CalculateSelectingProb(projectedStake, projectedMaxStaking, node.QOSScore)
		projectedTotalWeight += projectedWeight
		if node.Address == nodeAddress {
			projectedNodeWeight = projectedWeight
		}
	}
	if currentNodeWeight == 0 || projectedNodeWeight == 0 || projectedTotalWeight == 0 {
		return 0, 0
	}
	return currentNodeWeight / c.TotalWeight, projectedNodeWeight / projectedTotalWeight
}

func getNodeDelegationEmission(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) (*big.Int, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var row delegationEmissionGrantRow
	if err := db.WithContext(dbCtx).
		Model(&models.VestingDelegationEmissionDetail{}).
		Select("SUM(CAST(emission_amount AS DECIMAL(65,0))) as total_emission").
		Where("node_address = ?", nodeAddress).
		Where("start_time >= ? AND start_time < ?", start, end).
		Scan(&row).Error; err != nil {
		return nil, err
	}

	total, ok := parseDecimalInteger(row.TotalEmission)
	if !ok || total.Sign() <= 0 {
		return big.NewInt(0), nil
	}
	return total, nil
}

func parseDecimalInteger(raw string) (*big.Int, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return big.NewInt(0), false
	}
	if integer, fractional, ok := strings.Cut(value, "."); ok {
		if strings.TrimRight(fractional, "0") != "" {
			return big.NewInt(0), false
		}
		value = integer
	}
	parsed, ok := big.NewInt(0).SetString(value, 10)
	if !ok {
		return big.NewInt(0), false
	}
	return parsed, true
}

func getNodeOperatorEmission(ctx context.Context, db *gorm.DB, nodeAddress string, start, end time.Time) (*big.Int, error) {
	records, err := models.ListVestingRecordsByAddressAndTypeAndStartTimeRange(ctx, db, nodeAddress, models.VestingTypeNode, start, end)
	if err != nil {
		return nil, err
	}

	total := big.NewInt(0)
	for _, record := range records {
		total.Add(total, &record.TotalAmount.Int)
	}
	return total, nil
}

func RebuildDelegatedStakingNodeListSnapshots(ctx context.Context, db *gorm.DB, now time.Time, mainnetStartTime string) error {
	nodes, err := models.GetDelegatedNodes(ctx, db)
	if err != nil {
		return err
	}

	aprInputs, err := loadDelegationAPRInputs(ctx, db, nodes, now)
	if err != nil {
		return err
	}
	projectionContext, err := loadDelegationAPRProjectionContext(ctx, db, now)
	if err != nil {
		return err
	}
	snapshots := make([]models.DelegatedStakingNodeListSnapshot, 0, len(nodes))
	for _, node := range nodes {
		snapshot, err := buildDelegatedStakingNodeListSnapshot(ctx, db, *node, now, mainnetStartTime, aprInputs[node.Address], projectionContext)
		if err != nil {
			return err
		}
		if snapshot != nil {
			snapshots = append(snapshots, *snapshot)
		}
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.DelegatedStakingNodeListSnapshot{}).Error; err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return nil
		}
		return tx.CreateInBatches(snapshots, delegatedStakingNodeSnapshotBatchSize).Error
	})
}

func newDelegationAPRInput() *delegationAPRInput {
	return &delegationAPRInput{
		TotalDelegatorEarning:  big.NewInt(0),
		TotalDelegatorEmission: big.NewInt(0),
		TotalDelegatorStaking:  big.NewInt(0),
	}
}

func loadDelegationAPRInputs(ctx context.Context, db *gorm.DB, nodes []*models.Node, now time.Time) (map[string]*delegationAPRInput, error) {
	inputs := make(map[string]*delegationAPRInput, len(nodes))
	nodeAddresses := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if _, ok := inputs[node.Address]; ok {
			continue
		}
		inputs[node.Address] = newDelegationAPRInput()
		nodeAddresses = append(nodeAddresses, node.Address)
	}
	if len(nodeAddresses) == 0 {
		return inputs, nil
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	start, end, err := buildDelegationAPRRange(now, configuredDelegationAPRStartTime())
	if err != nil {
		return nil, err
	}

	earningRows := make([]delegationAPREarningRow, 0)
	if err := db.WithContext(dbCtx).
		Model(&models.NodeEarning{}).
		Select("node_address, SUM(CAST(delegator_earning AS DECIMAL(65,0))) as total_delegator_earning").
		Where("node_address IN ?", nodeAddresses).
		Where("time >= ? AND time < ?", start, end).
		Group("node_address").
		Scan(&earningRows).Error; err != nil {
		return nil, err
	}
	for _, row := range earningRows {
		amount, ok := parseDecimalInteger(row.TotalDelegatorEarning)
		if !ok || amount.Sign() <= 0 {
			continue
		}
		inputs[row.NodeAddress].TotalDelegatorEarning.Add(inputs[row.NodeAddress].TotalDelegatorEarning, amount)
	}

	emissionRows := make([]delegationEmissionGrantRow, 0)
	if err := db.WithContext(dbCtx).
		Model(&models.VestingDelegationEmissionDetail{}).
		Select("node_address, SUM(CAST(emission_amount AS DECIMAL(65,0))) as total_emission").
		Where("node_address IN ?", nodeAddresses).
		Where("start_time >= ? AND start_time < ?", start, end).
		Group("node_address").
		Scan(&emissionRows).Error; err != nil {
		return nil, err
	}
	for _, row := range emissionRows {
		amount, ok := parseDecimalInteger(row.TotalEmission)
		if !ok || amount.Sign() <= 0 {
			continue
		}
		inputs[row.NodeAddress].TotalDelegatorEmission.Add(inputs[row.NodeAddress].TotalDelegatorEmission, amount)
	}

	stakingRows := make([]delegationAPRStakingRow, 0)
	if err := db.WithContext(dbCtx).
		Model(&models.NodeStaking{}).
		Select("node_address, SUM(CAST(delegator_staking AS DECIMAL(65,0))) as total_delegator_staking, COUNT(*) as observation_days").
		Where("node_address IN ?", nodeAddresses).
		Where("time >= ? AND time < ?", start, end).
		Group("node_address").
		Scan(&stakingRows).Error; err != nil {
		return nil, err
	}
	for _, row := range stakingRows {
		amount, ok := parseDecimalInteger(row.TotalDelegatorStaking)
		if ok && amount.Sign() > 0 {
			inputs[row.NodeAddress].TotalDelegatorStaking.Add(inputs[row.NodeAddress].TotalDelegatorStaking, amount)
		}
		inputs[row.NodeAddress].ObservationDays = row.ObservationDays
	}

	return inputs, nil
}

func RefreshDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	appConfig := config.GetConfig()
	if appConfig == nil {
		return nil
	}

	node, err := models.GetNodeByAddress(ctx, db, nodeAddress)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return deleteDelegatedStakingNodeListSnapshot(ctx, db, nodeAddress)
		}
		return err
	}
	if node.DelegatorShare == 0 {
		return deleteDelegatedStakingNodeListSnapshot(ctx, db, nodeAddress)
	}

	now := time.Now().UTC()
	projectionContext, err := loadDelegationAPRProjectionContext(ctx, db, now)
	if err != nil {
		return err
	}
	snapshot, err := buildDelegatedStakingNodeListSnapshot(ctx, db, *node, now, appConfig.Dao.MainnetStartTime, nil, projectionContext)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return deleteDelegatedStakingNodeListSnapshot(ctx, db, nodeAddress)
	}

	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_address"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"network",
			"status",
			"status_group",
			"status_rank",
			"gpu_name",
			"gpu_vram",
			"version",
			"operator_emission_4w",
			"delegator_emission_4w",
			"operator_staking",
			"delegator_staking",
			"total_staking",
			"delegators_num",
			"prob_weight",
			"qos",
			"estimated_upcoming_operator_emission",
			"estimated_upcoming_delegator_emission",
			"delegation_apr_12m",
			"estimated_next_10k_delegation_apr",
			"estimated_next_100k_delegation_apr",
			"estimated_next_1m_delegation_apr",
			"apr_observation_days",
			"delegation_apr_updated_at",
			"updated_at",
		}),
	}).Create(snapshot).Error
}

func deleteDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	return db.WithContext(ctx).Where("node_address = ?", nodeAddress).Delete(&models.DelegatedStakingNodeListSnapshot{}).Error
}

func UpdateDelegatedStakingNodeListEmissionEstimates(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	rows := make([]delegatedStakingNodeListEstimateRow, 0)
	if err := db.WithContext(dbCtx).
		Model(&models.DelegatedStakingNodeListSnapshot{}).
		Select("node_address").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	return db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			operatorEstimate := GetNodeOperatorEmissionEstimate(row.NodeAddress)
			delegatorEstimate := GetNodeDelegationEmissionEstimate(row.NodeAddress)
			if err := tx.Model(&models.DelegatedStakingNodeListSnapshot{}).
				Where("node_address = ?", row.NodeAddress).
				Updates(map[string]interface{}{
					"estimated_upcoming_operator_emission":  models.BigInt{Int: *operatorEstimate.EstimatedEmission},
					"estimated_upcoming_delegator_emission": models.BigInt{Int: *delegatorEstimate.EstimatedEmission},
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
