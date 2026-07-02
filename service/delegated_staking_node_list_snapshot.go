package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
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
const delegatedStakingAPREmissionSource = "emission"

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
	return buildDelegatedStakingNodeListSnapshot(ctx, db, node, now, mainnetStartTime, nil)
}

func buildDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, node models.Node, now time.Time, mainnetStartTime string, aprInput *delegationAPRInput) (*models.DelegatedStakingNodeListSnapshot, error) {
	if node.DelegatorShare == 0 {
		return nil, nil
	}

	operatorEmission, err := getNodeOperatorEmission4w(ctx, db, node.Address, now, mainnetStartTime)
	if err != nil {
		return nil, err
	}

	lockedEmission := GetNodeLockedVestingAmount(node.Address, now)
	operatorStaking := big.NewInt(0).Add(&node.StakeAmount.Int, lockedEmission)
	delegatorStaking := GetNodeTotalStakeAmount(node.Address, node.Network)
	totalStaking := big.NewInt(0).Add(operatorStaking, delegatorStaking)
	delegatorsNum := GetDelegatorCountOfNode(node.Address, node.Network)
	selectingProb := CalculateNodeSelectingProb(node, now)
	statusGroup, statusRank := BuildDelegatedStakingNodeStatusGroup(node.Status)
	var delegationAPR12m float64
	var aprObservationDays uint32
	if aprInput == nil {
		var err error
		delegationAPR12m, aprObservationDays, err = CalculateNodeDelegationAPR12m(ctx, db, node.Address, now)
		if err != nil {
			return nil, err
		}
	} else {
		delegationAPR12m, aprObservationDays = calculateDelegationAPR(aprInput)
	}

	return &models.DelegatedStakingNodeListSnapshot{
		NodeAddress:            node.Address,
		Network:                node.Network,
		Status:                 node.Status,
		StatusGroup:            statusGroup,
		StatusRank:             statusRank,
		GPUName:                node.GPUName,
		GPUVram:                node.GPUVram,
		Version:                buildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion),
		OperatorEmission4w:     models.BigInt{Int: *operatorEmission},
		OperatorStaking:        models.BigInt{Int: *operatorStaking},
		DelegatorStaking:       models.BigInt{Int: *delegatorStaking},
		TotalStaking:           models.BigInt{Int: *totalStaking},
		DelegatorsNum:          uint64(delegatorsNum),
		ProbWeight:             selectingProb.ProbWeight,
		QOS:                    selectingProb.QOSScore,
		DelegationApr12m:       delegationAPR12m,
		AprObservationDays:     aprObservationDays,
		DelegationAprUpdatedAt: now.UTC(),
	}, nil
}

func CalculateNodeDelegationAPR12m(ctx context.Context, db *gorm.DB, nodeAddress string, now time.Time) (float64, uint32, error) {
	end := now.UTC()
	start := end.AddDate(0, -delegatedStakingAPRObservationMonths, 0)

	input, err := loadNodeDelegationAPRInput(ctx, db, nodeAddress, start, end)
	if err != nil {
		return 0, 0, err
	}
	apr, observationDays := calculateDelegationAPR(input)
	return apr, observationDays, nil
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

func getNodeOperatorEmission4w(ctx context.Context, db *gorm.DB, nodeAddress string, now time.Time, mainnetStartTime string) (*big.Int, error) {
	chartRange, err := BuildEmissionChartRange(now, mainnetStartTime, 4)
	if err != nil {
		return nil, err
	}
	records, err := models.ListVestingRecordsByAddressAndTypeAndStartTimeRange(ctx, db, nodeAddress, models.VestingTypeNode, chartRange.RangeStart, chartRange.RangeEnd)
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
	snapshots := make([]models.DelegatedStakingNodeListSnapshot, 0, len(nodes))
	for _, node := range nodes {
		snapshot, err := buildDelegatedStakingNodeListSnapshot(ctx, db, *node, now, mainnetStartTime, aprInputs[node.Address])
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
	end := now.UTC()
	start := end.AddDate(0, -delegatedStakingAPRObservationMonths, 0)

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

	snapshot, err := BuildDelegatedStakingNodeListSnapshot(ctx, db, *node, time.Now().UTC(), appConfig.Dao.MainnetStartTime)
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
			"operator_staking",
			"delegator_staking",
			"total_staking",
			"delegators_num",
			"prob_weight",
			"qos",
			"delegation_apr_12m",
			"apr_observation_days",
			"delegation_apr_updated_at",
			"updated_at",
		}),
	}).Create(snapshot).Error
}

func deleteDelegatedStakingNodeListSnapshot(ctx context.Context, db *gorm.DB, nodeAddress string) error {
	return db.WithContext(ctx).Where("node_address = ?", nodeAddress).Delete(&models.DelegatedStakingNodeListSnapshot{}).Error
}
