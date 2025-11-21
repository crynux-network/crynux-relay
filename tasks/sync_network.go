package tasks

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"math/big"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func StartSyncNetwork(ctx context.Context) {
	duration := 3 * time.Minute
	ticker := time.NewTicker(duration)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			ticker.Stop()
			log.Errorf("SyncNetwork: stop syncing network due to %v", err)
			return
		case <-ticker.C:
			func() {
				ctx1, cancel := context.WithTimeout(ctx, duration)
				defer cancel()
				log.Infof("SyncNetwork: start syncing network")
				if err := SyncNetwork(ctx1); err != nil {
					log.Errorf("SyncNetwork: sync network error %v", err)
				}
				log.Infof("SyncNetwork: end syncing network")
			}()
		}
	}
}

func getNodeData(ctx context.Context, db *gorm.DB, offset, limit int) ([]models.NetworkNodeData, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodes []models.Node
	var nodesData []models.NetworkNodeData
	if err := db.WithContext(dbCtx).Model(&models.NetworkNodeData{}).Order("id").Offset(offset).Limit(limit).Find(&nodesData).Error; err != nil {
		return nil, err
	}
	var nodeAddresses []string
	for _, nodeData := range nodesData {
		nodeAddresses = append(nodeAddresses, nodeData.Address)
	}
	if err := db.WithContext(dbCtx).Model(&models.Node{}).Where("address IN (?)", nodeAddresses).Where("status != ?", models.NodeStatusQuit).Find(&nodes).Error; err != nil {
		return nil, err
	}

	nodesMap := make(map[string]models.Node)
	for _, node := range nodes {
		nodesMap[node.Address] = node
	}

	for i, nodeData := range nodesData {
		taskFee, err := service.GetTaskFee(ctx, db, nodeData.Address)
		if err != nil {
			log.Errorf("SyncNetwork: error getting task fee %v", err)
			return nil, err
		}
		nodesData[i].Balance = models.BigInt{Int: *taskFee}
		node, ok := nodesMap[nodeData.Address]
		if ok {
			nodesData[i].QoS = node.QOSScore
			nodesData[i].Staking = models.BigInt{Int: *new(big.Int).Add(&node.StakeAmount.Int, service.GetNodeTotalStakeAmount(node.Address, node.Network))}
		}
	}
	return nodesData, nil
}

func syncNodeNumber(ctx context.Context) error {
	busyNodes, err := models.GetBusyNodeCount(ctx, config.GetDB())
	if err != nil {
		log.Errorf("SyncNetwork: error getting busy nodes count %v", err)
		return err
	}
	allNodes, err := models.GetAllNodeCount(ctx, config.GetDB())
	if err != nil {
		log.Errorf("SyncNetwork: error getting all nodes count %v", err)
		return err
	}
	activeNodes, err := models.GetActiveNodeCount(ctx, config.GetDB())
	if err != nil {
		log.Errorf("SyncNetwork: error getting active nodes count %v", err)
		return err
	}

	nodeNumber := models.NetworkNodeNumber{
		BusyNodes:   uint64(busyNodes),
		AllNodes:    uint64(allNodes),
		ActiveNodes: uint64(activeNodes),
	}

	if err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return config.GetDB().WithContext(dbCtx).Model(&nodeNumber).Where("id = ?", 1).Assign(nodeNumber).FirstOrCreate(&models.NetworkNodeNumber{}).Error
	}(); err != nil {
		log.Errorf("SyncNetwork: error update NetworkNodeNumber %v", err)
		return err
	}
	return nil
}

func syncTaskNumber(ctx context.Context) error {
	runningTasks, err := models.GetRunningTaskCount(ctx, config.GetDB())
	if err != nil {
		log.Errorf("SyncNetwork: error getting running task count %v", err)
		return err
	}
	queuedTasks, err := models.GetQueuedTaskCount(ctx, config.GetDB())
	if err != nil {
		log.Errorf("SyncNetwork: error getting queued task count %v", err)
		return err
	}

	taskNumber := models.NetworkTaskNumber{
		RunningTasks: uint64(runningTasks),
		QueuedTasks:  uint64(queuedTasks),
	}

	if err := func() error {
		dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return config.GetDB().WithContext(dbCtx).Model(&taskNumber).Where("id = ?", 1).Assign(taskNumber).FirstOrCreate(&models.NetworkTaskNumber{}).Error
	}(); err != nil {
		log.Errorf("SyncNetwork: error update NetworkNodeNumber")
		log.Error(err)
		return err
	}
	return nil
}

func syncNodeData(ctx context.Context) error {
	limit := 100
	offset := 0
	var totalGFLOPS float64 = 0

	workerCount := 10
	semaphore := make(chan struct{}, workerCount)

	var allNodeDatas []models.NetworkNodeData
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	for {
		nodeDatas, err := getNodeData(ctx, config.GetDB(), offset, limit)
		if err != nil {
			log.Errorf("SyncNetwork: error getting nodes data %v", err)
			return err
		}

		if len(nodeDatas) == 0 {
			break
		}
		allNodeDatas = append(allNodeDatas, nodeDatas...)

		wg.Add(1)
		go func(ctx context.Context, batchData []models.NetworkNodeData) {
			defer wg.Done()

			ctx1, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				select {
				case errChan <- ctx.Err():
				default:
				}
				return
			}

			if err := batchUpsertNodeData(ctx1, batchData); err != nil {
				log.Errorf("SyncNetwork: error batch upserting node data: %v", err)
				select {
				case errChan <- err:
				default:
				}
				return
			}
		}(ctx, nodeDatas)

		offset += limit
	}

	wg.Add(1)
	go func(ctx context.Context) {
		ctx1, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		defer wg.Done()
		for _, data := range allNodeDatas {
			totalGFLOPS += models.GetGPUGFLOPS(data.CardModel)
		}

		networkFLOPS := models.NetworkFLOPS{GFLOPS: totalGFLOPS}
		if err := config.GetDB().WithContext(ctx1).Model(&networkFLOPS).Where("id = ?", 1).Assign(networkFLOPS).FirstOrCreate(&models.NetworkFLOPS{}).Error; err != nil {
			log.Errorf("SyncNetwork: error updating NetworkFLOPS %v", err)
			select {
			case errChan <- err:
			default:
			}
			return
		}
	}(ctx)

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	return nil
}

func batchUpsertNodeData(ctx context.Context, nodeDatas []models.NetworkNodeData) error {
	if len(nodeDatas) == 0 {
		return nil
	}

	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return config.GetDB().WithContext(dbCtx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		DoUpdates: clause.AssignmentColumns([]string{"balance", "qo_s", "staking", "updated_at"}),
	}).CreateInBatches(nodeDatas, len(nodeDatas)).Error
}

func SyncNetwork(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	wg.Add(3)

	go func() {
		defer wg.Done()
		err := syncNodeNumber(ctx)
		if err != nil {
			log.Errorf("SyncNetwork: error syncing node number %v", err)
		}
		errChan <- err
	}()

	go func() {
		defer wg.Done()
		err := syncTaskNumber(ctx)
		if err != nil {
			log.Errorf("SyncNetwork: error syncing task number %v", err)
		}
		errChan <- err
	}()

	go func() {
		defer wg.Done()
		err := syncNodeData(ctx)
		if err != nil {
			log.Errorf("SyncNetwork: error syncing node data %v", err)
		}
		errChan <- err
	}()

	wg.Wait()
	close(errChan)
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}
