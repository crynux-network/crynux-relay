package tasks

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var initStartTime time.Time = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
var statsDuration time.Duration = time.Hour

func getTaskCounts(ctx context.Context, start, end time.Time) ([]*models.TaskCount, error) {
	var results []*models.TaskCount

	taskTypes := []models.TaskType{models.TaskTypeSD, models.TaskTypeLLM, models.TaskTypeSDFTLora}

	for _, taskType := range taskTypes {
		var successCount, abortedCount int64

		err := func() error {
			dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := config.GetDB().WithContext(dbCtx).Model(&models.InferenceTask{}).
				Where("created_at >= ?", start).Where("created_at < ?", end).
				Where("task_type = ?", taskType).
				Where("(status = ? OR status = ?)", models.TaskEndAborted, models.TaskEndInvalidated).
				Count(&abortedCount).Error; err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			log.Errorf("Stats: get %d type aborted task count error: %v", taskType, err)
			return nil, err
		}

		err = func() error {
			dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := config.GetDB().WithContext(dbCtx).Model(&models.InferenceTask{}).
				Where("created_at >= ?", start).Where("created_at < ?", end).
				Where("task_type = ?", taskType).
				Where("(status = ? OR status = ?)", models.TaskEndSuccess, models.TaskEndGroupRefund).
				Count(&successCount).Error; err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			log.Errorf("Stats: get %d type success task count error: %v", taskType, err)
			return nil, err
		}

		totalCount := successCount + abortedCount

		taskCount := models.TaskCount{
			Start:        start,
			End:          end,
			TaskType:     taskType,
			TotalCount:   totalCount,
			SuccessCount: successCount,
			AbortedCount: abortedCount,
		}

		results = append(results, &taskCount)
	}
	return results, nil
}

func statsTaskCount(ctx context.Context) error {
	now := time.Now().UTC()
	taskCount := models.TaskCount{}
	err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return config.GetDB().WithContext(dbCtx).Model(&models.TaskCount{}).Last(&taskCount).Error
	}()
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Errorf("Stats: get last TaskCount error: %v", err)
		return err
	}

	var start time.Time
	if taskCount.ID > 0 {
		start = taskCount.End
	} else {
		start = initStartTime
	}

	for {
		end := start.Add(statsDuration)
		if end.Sub(now) > 0 {
			break
		}
		taskCounts, err := getTaskCounts(ctx, start, end)
		if err != nil {
			return err
		}
		if len(taskCounts) > 0 {
			err := func() error {
				dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				return config.GetDB().WithContext(dbCtx).Create(taskCounts).Error
			}()
			if err != nil {
				log.Errorf("Stats: create TaskCount error: %v", err)
				return err
			}
		}
		log.Infof("Stats: stats TaskCount success %s", end.Format(time.RFC3339))
		start = end
	}

	return nil
}

func StartStatsTaskCount(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			ticker.Stop()
			log.Infof("Stats: stop counting task count due to %v", err)
			return
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsTaskCount(ctx1); err != nil {
					log.Errorf("Stats: stats task count error %v", err)
				}
			}()
		}
	}
}

func getTaskExecutionTimeCount(ctx context.Context, start, end time.Time) ([]*models.TaskExecutionTimeCount, error) {
	var results []*models.TaskExecutionTimeCount

	taskTypes := []models.TaskType{models.TaskTypeSD, models.TaskTypeLLM, models.TaskTypeSDFTLora}
	modelSwitchedEnums := []bool{false, true}
	binSize := 5
	for _, taskType := range taskTypes {
		for _, modelSwitched := range modelSwitchedEnums {
			rows, err := func() (*sql.Rows, error) {
				dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				subQuery := config.GetDB().Table("inference_tasks").
					Select("id, CAST(TIMESTAMPDIFF(SECOND, start_time, score_ready_time) / ? AS SIGNED) AS time", binSize).
					Where("created_at >= ?", start).Where("created_at < ?", end).
					Where("task_type = ?", taskType).
					Where("model_swtiched = ?", modelSwitched).
					Where("score_ready_time IS NOT NULL")
				return config.GetDB().WithContext(dbCtx).
					Table("(?) AS s", subQuery).
					Select("s.time * ? as T, COUNT(s.id) AS count", binSize).
					Where("s.time >= 0").
					Group("T").Order("T").Rows()
			}()

			if err != nil {
				log.Errorf("Stats: get %d type task execution time error: %v", taskType, err)
				return nil, err
			}
			defer rows.Close()
			var seconds, count int64
			for rows.Next() {
				rows.Scan(&seconds, &count)
				results = append(results, &models.TaskExecutionTimeCount{
					Start:         start,
					End:           end,
					TaskType:      taskType,
					Seconds:       seconds,
					Count:         count,
					ModelSwitched: modelSwitched,
				})
			}
		}
	}
	return results, nil
}

func statsTaskExecutionTimeCount(ctx context.Context) error {
	now := time.Now().UTC()

	taskExecutionTimeCount := models.TaskExecutionTimeCount{}
	err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return config.GetDB().WithContext(dbCtx).Model(&models.TaskExecutionTimeCount{}).Last(&taskExecutionTimeCount).Error
	}()
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Errorf("Stats: get last TaskExecutionTimeCount error: %v", err)
		return err
	}
	var start time.Time
	if taskExecutionTimeCount.ID > 0 {
		start = taskExecutionTimeCount.End
	} else {
		start = initStartTime
	}

	for {
		end := start.Add(statsDuration)
		if end.Sub(now) > 0 {
			break
		}

		taskExecutionTimeCounts, err := getTaskExecutionTimeCount(ctx, start, end)
		if err != nil {
			return err
		}
		if len(taskExecutionTimeCounts) > 0 {
			err := func() error {
				dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				return config.GetDB().WithContext(dbCtx).Create(taskExecutionTimeCounts).Error
			}()
			if err != nil {
				log.Errorf("Stats: create TaskExecutionTimeCount error: %v", err)
				return err
			}
		}
		log.Infof("Stats: stats TaskExecutionTimeCount success: %s", end.Format(time.RFC3339))
		start = end
	}

	return nil
}

func StartStatsTaskExecutionTimeCount(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Errorf("Stats: stop counting task execution time count due to %v", err)
			ticker.Stop()
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsTaskExecutionTimeCount(ctx1); err != nil {
					log.Errorf("Stats: stats task execution time count error %v", err)
				}
			}()
		}
	}
}

func getTaskUploadResultTimeCount(ctx context.Context, start, end time.Time) ([]*models.TaskUploadResultTimeCount, error) {
	var results []*models.TaskUploadResultTimeCount

	taskTypes := []models.TaskType{models.TaskTypeSD, models.TaskTypeLLM, models.TaskTypeSDFTLora}
	binSize := 5
	for _, taskType := range taskTypes {
		rows, err := func() (*sql.Rows, error) {
			dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			subQuery := config.GetDB().Table("inference_tasks").
				Select("id, CAST(TIMESTAMPDIFF(SECOND, validated_time, result_uploaded_time) / ? AS SIGNED) AS time", binSize).
				Where("created_at >= ?", start).Where("created_at < ?", end).
				Where("task_type = ?", taskType).
				Where("result_uploaded_time IS NOT NULL")
			return config.GetDB().WithContext(dbCtx).
				Table("(?) AS s", subQuery).
				Select("s.time * ? as T, COUNT(s.id) AS count", binSize).
				Where("s.time >= 0").
				Group("T").Order("T").Rows()
		}()
		if err != nil {
			log.Errorf("Stats: get %d type task result upload time error: %v", taskType, err)
			return nil, err
		}
		defer rows.Close()
		var seconds, count int64
		for rows.Next() {
			rows.Scan(&seconds, &count)
			results = append(results, &models.TaskUploadResultTimeCount{
				Start:    start,
				End:      end,
				TaskType: taskType,
				Seconds:  seconds,
				Count:    count,
			})
		}
	}
	return results, nil
}

func statsTaskUploadResultTimeCount(ctx context.Context) error {
	now := time.Now().UTC()
	taskUploadResultTimeCount := models.TaskUploadResultTimeCount{}

	err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return config.GetDB().WithContext(dbCtx).Model(&models.TaskUploadResultTimeCount{}).Last(&taskUploadResultTimeCount).Error
	}()
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Errorf("Stats: get last TaskUploadResultTime error: %v", err)
		return err
	}
	var start time.Time
	if taskUploadResultTimeCount.ID > 0 {
		start = taskUploadResultTimeCount.End
	} else {
		start = initStartTime
	}

	for {
		end := start.Add(statsDuration)
		if end.Sub(now) > 0 {
			break
		}

		taskUploadResultTimeCounts, err := getTaskUploadResultTimeCount(ctx, start, end)
		if err != nil {
			return err
		}
		if len(taskUploadResultTimeCounts) > 0 {
			err := func() error {
				dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				return config.GetDB().WithContext(dbCtx).Create(taskUploadResultTimeCounts).Error
			}()
			if err != nil {
				log.Errorf("Stats: create TaskUploadResultTimeCount error: %v", err)
				return err
			}
		}
		log.Infof("Stats: stats TaskUploadResultTimeCount success: %s", end.Format(time.RFC3339))
		start = end
	}

	return nil
}

func StartStatsTaskUploadResultTimeCount(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Errorf("Stats: stop counting task upload result time count due to %v", err)
			ticker.Stop()
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsTaskUploadResultTimeCount(ctx1); err != nil {
					log.Errorf("Stats: stats task upload result time count error %v", err)
				}
			}()
		}
	}
}

func getTaskWaitingTimeCount(ctx context.Context, start, end time.Time) ([]*models.TaskWaitingTimeCount, error) {
	var results []*models.TaskWaitingTimeCount

	taskTypes := []models.TaskType{models.TaskTypeSD, models.TaskTypeLLM, models.TaskTypeSDFTLora}
	binSize := 5
	for _, taskType := range taskTypes {
		rows, err := func() (*sql.Rows, error) {
			dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			subQuery := config.GetDB().Table("inference_tasks").
				Select("id, CAST(TIMESTAMPDIFF(SECOND, create_time, start_time) / ? AS SIGNED) AS time", binSize).
				Where("created_at >= ?", start).Where("created_at < ?", end).
				Where("task_type = ?", taskType).
				Where("start_time IS NOT NULL")
			return config.GetDB().WithContext(dbCtx).Table("(?) AS s", subQuery).
				Select("s.time * ? as T, COUNT(s.id) AS count", binSize).
				Where("s.time >= 0").
				Group("T").Order("T").Rows()
		}()
		if err != nil {
			log.Errorf("Stats: get %d type task result upload time error: %v", taskType, err)
			return nil, err
		}
		defer rows.Close()
		var seconds, count int64
		for rows.Next() {
			rows.Scan(&seconds, &count)
			results = append(results, &models.TaskWaitingTimeCount{
				Start:    start,
				End:      end,
				TaskType: taskType,
				Seconds:  seconds,
				Count:    count,
			})
		}
	}
	return results, nil
}

func statsTaskWaitingTimeCount(ctx context.Context) error {
	now := time.Now().UTC()

	taskWaitingTimeCount := models.TaskWaitingTimeCount{}
	err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return config.GetDB().WithContext(dbCtx).Model(&models.TaskWaitingTimeCount{}).Last(&taskWaitingTimeCount).Error
	}()
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Errorf("Stats: get last TaskWaitingTimeCount error: %v", err)
		return err
	}
	var start time.Time
	if taskWaitingTimeCount.ID > 0 {
		start = taskWaitingTimeCount.End
	} else {
		start = initStartTime
	}

	for {
		end := start.Add(statsDuration)
		if end.Sub(now) > 0 {
			break
		}

		taskWaitingTimeCounts, err := getTaskWaitingTimeCount(ctx, start, end)
		if err != nil {
			return err
		}
		if len(taskWaitingTimeCounts) > 0 {
			err := func() error {
				dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				return config.GetDB().WithContext(dbCtx).Create(taskWaitingTimeCounts).Error
			}()
			if err != nil {
				log.Errorf("Stats: create TaskWaitingTimeCount error: %v", err)
				return err
			}
		}
		log.Infof("Stats: stats TaskWaitingTimeCount success: %s", end.Format(time.RFC3339))
		start = end
	}

	return nil
}

func StartStatsTaskWaitingTimeCount(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Errorf("Stats: stop counting task waiting time count due to %v", err)
			ticker.Stop()
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsTaskWaitingTimeCount(ctx1); err != nil {
					log.Errorf("Stats: stats task waiting time count error %v", err)
				}
			}()
		}
	}
}

func getAllNodes(ctx context.Context, db *gorm.DB) ([]models.Node, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var allNodes []models.Node
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		limit := 1000
		offset := 0
		for {
			var nodes []models.Node
			if err := tx.Model(&models.Node{}).Order("id").Limit(limit).Offset(offset).Find(&nodes).Error; err != nil {
				return err
			}
			if len(nodes) == 0 {
				break
			}
			allNodes = append(allNodes, nodes...)
			offset += len(nodes)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return allNodes, nil
}

func batchCreateNodeScores(ctx context.Context, db *gorm.DB, nodes []models.Node, t time.Time) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var nodeAddresses []string
	for _, node := range nodes {
		nodeAddresses = append(nodeAddresses, node.Address)
	}

	var existedNodeScores []models.NodeScore
	var newNodeScores []models.NodeScore

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.NodeScore{}).Where("node_address in (?)", nodeAddresses).Where("time = ?", t).Find(&existedNodeScores).Error; err != nil {
			return err
		}

		existedNodeScoreMap := make(map[string]models.NodeScore)
		for _, ns := range existedNodeScores {
			existedNodeScoreMap[ns.NodeAddress] = ns
		}

		var stakingScoreCase, qosScoreCase, probWeightCase strings.Builder
		stakingScoreCase.WriteString("CASE ")
		qosScoreCase.WriteString("CASE ")
		probWeightCase.WriteString("CASE ")

		for _, node := range nodes {
			totalStakeAmount := big.NewInt(0)
			if node.Status != models.NodeStatusQuit {
				totalStakeAmount = new(big.Int).Add(&node.StakeAmount.Int, service.GetNodeTotalStakeAmount(node.Address, node.Network))
			}
			stakingScore, qosScore, probWeight := service.CalculateSelectingProb(totalStakeAmount, service.GetMaxStaking(), node.QOSScore, service.GetMaxQosScore())
			if _, ok := existedNodeScoreMap[node.Address]; ok {
				stakingScoreCase.WriteString(fmt.Sprintf("WHEN node_address = '%s' AND time = '%s' THEN %f ", node.Address, t.Format("2006-01-02 15:04:05.000"), stakingScore))
				qosScoreCase.WriteString(fmt.Sprintf("WHEN node_address = '%s' AND time = '%s' THEN %f ", node.Address, t.Format("2006-01-02 15:04:05.000"), qosScore))
				probWeightCase.WriteString(fmt.Sprintf("WHEN node_address = '%s' AND time = '%s' THEN %f ", node.Address, t.Format("2006-01-02 15:04:05.000"), probWeight))
			} else {
				nodeScore := models.NodeScore{
					NodeAddress:  node.Address,
					Time:         t,
					StakingScore: stakingScore,
					QOSScore:     qosScore,
					ProbWeight:   probWeight,
				}
				newNodeScores = append(newNodeScores, nodeScore)
			}
		}

		stakingScoreCase.WriteString("END")
		qosScoreCase.WriteString("END")
		probWeightCase.WriteString("END")

		if len(existedNodeScores) > 0 {
			if err := tx.Model(&models.NodeScore{}).
				Where("node_address in (?)", nodeAddresses).
				Where("time = ?", t).
				Updates(map[string]interface{}{
					"staking_score": gorm.Expr(stakingScoreCase.String()),
					"qos_score":     gorm.Expr(qosScoreCase.String()),
					"prob_weight":   gorm.Expr(probWeightCase.String()),
				}).Error; err != nil {
				return err
			}
		}

		if len(newNodeScores) > 0 {
			if err := tx.CreateInBatches(newNodeScores, 100).Error; err != nil {
				return err
			}
		}
		return nil

	}); err != nil {
		return err
	}

	return nil
}

func statsNodeScores(ctx context.Context, interval time.Duration) error {
	t := time.Now().UTC().Truncate(interval)
	nodes, err := getAllNodes(ctx, config.GetDB())
	if err != nil {
		return err
	}
	if err := batchCreateNodeScores(ctx, config.GetDB(), nodes, t); err != nil {
		return err
	}
	return nil
}

func StartStatsNodeScores(ctx context.Context) {
	interval := 4 * time.Hour
	if err := statsNodeScores(ctx, interval); err != nil {
		log.Errorf("Stats: initial stats node scores error %v", err)
	}

	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Errorf("Stats: stop counting node scores due to %v", err)
			ticker.Stop()
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsNodeScores(ctx1, interval); err != nil {
					log.Errorf("Stats: stats node scores error %v", err)
				}
			}()
		}
	}
}

func batchCreateNodeStakings(ctx context.Context, db *gorm.DB, nodes []models.Node, t time.Time) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var existedNodeStakings []models.NodeStaking
		var newNodeStakings []models.NodeStaking

		var nodeAddresses []string
		for _, node := range nodes {
			nodeAddresses = append(nodeAddresses, node.Address)
		}

		if err := tx.Model(&models.NodeStaking{}).Where("node_address in (?)", nodeAddresses).Where("time = ?", t).Find(&existedNodeStakings).Error; err != nil {
			return err
		}

		existedNodeStakingMap := make(map[string]models.NodeStaking)
		for _, ns := range existedNodeStakings {
			existedNodeStakingMap[ns.NodeAddress] = ns
		}

		var operatorStakingCase, delegatorStakingCase strings.Builder
		operatorStakingCase.WriteString("CASE ")
		delegatorStakingCase.WriteString("CASE ")

		for _, node := range nodes {
			nodeStaking := models.NodeStaking{
				NodeAddress:      node.Address,
				Time:             t,
				OperatorStaking:  models.BigInt{Int: *big.NewInt(0)},
				DelegatorStaking: models.BigInt{Int: *big.NewInt(0)},
			}
			if node.Status != models.NodeStatusQuit {
				nodeStaking.OperatorStaking = node.StakeAmount
				delegatorStaking := service.GetNodeTotalStakeAmount(node.Address, node.Network)
				nodeStaking.DelegatorStaking = models.BigInt{Int: *delegatorStaking}
			}
			if _, ok := existedNodeStakingMap[node.Address]; ok {
				operatorStakingCase.WriteString(fmt.Sprintf("WHEN node_address = '%s' AND time = '%s' THEN '%s' ", node.Address, t.Format("2006-01-02 15:04:05.000"), nodeStaking.OperatorStaking.String()))
				delegatorStakingCase.WriteString(fmt.Sprintf("WHEN node_address = '%s' AND time = '%s' THEN '%s' ", node.Address, t.Format("2006-01-02 15:04:05.000"), nodeStaking.DelegatorStaking.String()))
			} else {
				newNodeStakings = append(newNodeStakings, nodeStaking)
			}
		}

		operatorStakingCase.WriteString("END")
		delegatorStakingCase.WriteString("END")

		if len(existedNodeStakings) > 0 {
			if err := tx.Model(&models.NodeStaking{}).
				Where("node_address in (?)", nodeAddresses).
				Where("time = ?", t).
				Updates(map[string]interface{}{
					"operator_staking":  gorm.Expr(operatorStakingCase.String()),
					"delegator_staking": gorm.Expr(delegatorStakingCase.String()),
				}).Error; err != nil {
				return err
			}
		}

		if len(newNodeStakings) > 0 {
			if err := tx.CreateInBatches(newNodeStakings, 100).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func statsNodeStakings(ctx context.Context, interval time.Duration) error {
	t := time.Now().UTC().Truncate(interval)
	nodes, err := getAllNodes(ctx, config.GetDB())
	if err != nil {
		return err
	}
	if err := batchCreateNodeStakings(ctx, config.GetDB(), nodes, t); err != nil {
		return err
	}
	return nil
}

func StartStatsNodeStakings(ctx context.Context) {
	interval := 4 * time.Hour
	if err := statsNodeStakings(ctx, interval); err != nil {
		log.Errorf("Stats: initial stats node stakings error %v", err)
	}
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Errorf("Stats: stop counting node stakings due to %v", err)
			ticker.Stop()
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsNodeStakings(ctx1, interval); err != nil {
					log.Errorf("Stats: stats node stakings error %v", err)
				}
			}()
		}
	}
}

func batchCreateNodeDelegatorCount(ctx context.Context, db *gorm.DB, nodes []models.Node, t time.Time) error {
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var existedNodeDelegatorCounts []models.NodeDelegatorCount
		var newNodeDelegatorCounts []models.NodeDelegatorCount

		var nodeAddresses []string
		for _, node := range nodes {
			nodeAddresses = append(nodeAddresses, node.Address)
		}

		if err := tx.Model(&models.NodeDelegatorCount{}).Where("node_address in (?)", nodeAddresses).Where("time = ?", t).Find(&existedNodeDelegatorCounts).Error; err != nil {
			return err
		}

		existedNodeDelegatorCountMap := make(map[string]models.NodeDelegatorCount)
		for _, ndc := range existedNodeDelegatorCounts {
			existedNodeDelegatorCountMap[ndc.NodeAddress] = ndc
		}

		var cases strings.Builder
		cases.WriteString("CASE ")

		for _, node := range nodes {
			delegatorCount := service.GetDelegatorCountOfNode(node.Address, node.Network)
			if _, ok := existedNodeDelegatorCountMap[node.Address]; ok {
				cases.WriteString(fmt.Sprintf("WHEN node_address = '%s' AND time = '%s' THEN %d ", node.Address, t.Format("2006-01-02 15:04:05.000"), delegatorCount))
			} else {
				nodeDelegatorCount := models.NodeDelegatorCount{
					NodeAddress: node.Address,
					Time:        t,
					Count:       uint64(delegatorCount),
				}
				newNodeDelegatorCounts = append(newNodeDelegatorCounts, nodeDelegatorCount)
			}
		}

		cases.WriteString("END")

		if len(existedNodeDelegatorCounts) > 0 {
			if err := tx.Model(&models.NodeDelegatorCount{}).
				Where("node_address in (?)", nodeAddresses).
				Where("time = ?", t).
				Updates(map[string]interface{}{
					"count": gorm.Expr(cases.String()),
				}).Error; err != nil {
				return err
			}
		}

		if len(newNodeDelegatorCounts) > 0 {
			if err := tx.CreateInBatches(newNodeDelegatorCounts, 100).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func statsNodeDelegatorCount(ctx context.Context, interval time.Duration) error {
	t := time.Now().UTC().Truncate(interval)
	nodes, err := getAllNodes(ctx, config.GetDB())
	if err != nil {
		return err
	}
	if err := batchCreateNodeDelegatorCount(ctx, config.GetDB(), nodes, t); err != nil {
		return err
	}
	return nil
}

func StartStatsNodeDelegatorCount(ctx context.Context) {
	interval := 24 * time.Hour
	if err := statsNodeDelegatorCount(ctx, interval); err != nil {
		log.Errorf("Stats: initial stats node delegator count error %v", err)
	}
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Errorf("Stats: stop counting node delegator count due to %v", err)
			ticker.Stop()
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, 5*time.Minute)
				defer cancel()
				if err := statsNodeDelegatorCount(ctx1, interval); err != nil {
					log.Errorf("Stats: stats node delegator count error %v", err)
				}
			}()
		}
	}
}
