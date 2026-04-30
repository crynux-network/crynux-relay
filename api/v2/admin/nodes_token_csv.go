package admin

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"crynux_relay/utils"
	"encoding/csv"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	nodesTokenCSVChainRequestBatchSize = 10
	nodesTokenCSVChainRequestPause     = time.Second
	nodesTokenCSVDymensionNetwork      = "dymension"
	nodesTokenCSVNearNetwork           = "near"
)

var nodesTokenCSVExportState = struct {
	sync.Mutex
	running bool
}{}

type nodesTokenCSVRow struct {
	Address             string
	DymChainBalance     *big.Int
	NearChainBalance    *big.Int
	RelayAccountBalance *big.Int
	Staking             *big.Int
	Total               *big.Int
}

type nodesTokenCSVChainRequestLimiter struct {
	count int
}

func ExportNodesTokenCSV(c *gin.Context) {
	if !startNodesTokenCSVExport() {
		c.JSON(http.StatusAccepted, gin.H{
			"message": "nodes token export is already running",
		})
		return
	}

	go func() {
		defer finishNodesTokenCSVExport()
		exportNodesTokenCSV(context.Background())
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "nodes token export is running",
	})
}

func startNodesTokenCSVExport() bool {
	nodesTokenCSVExportState.Lock()
	defer nodesTokenCSVExportState.Unlock()
	if nodesTokenCSVExportState.running {
		return false
	}
	nodesTokenCSVExportState.running = true
	return true
}

func finishNodesTokenCSVExport() {
	nodesTokenCSVExportState.Lock()
	defer nodesTokenCSVExportState.Unlock()
	nodesTokenCSVExportState.running = false
}

func exportNodesTokenCSV(ctx context.Context) {
	logger := getAdminLogger()
	logger.Info("Start exporting nodes token CSV")

	var nodes []models.Node
	if err := config.GetDB().WithContext(ctx).Model(&models.Node{}).Order("address ASC").Find(&nodes).Error; err != nil {
		logger.WithError(err).Error("Failed to load nodes for token CSV export")
		return
	}

	totalWallets := len(nodes)
	logger.WithField("total_wallets", totalWallets).Info("Loaded wallets for nodes token CSV export")

	rows := make([]nodesTokenCSVRow, 0, totalWallets)
	limiter := &nodesTokenCSVChainRequestLimiter{}
	for i, node := range nodes {
		row, err := buildNodesTokenCSVRow(ctx, node, limiter)
		if err != nil {
			logger.WithError(err).WithFields(log.Fields{
				"wallet":            node.Address,
				"total_wallets":     totalWallets,
				"remaining_wallets": totalWallets - i,
			}).Error("Failed to export wallet token data")
			return
		}
		rows = append(rows, row)

		logger.WithFields(log.Fields{
			"wallet":            node.Address,
			"total_wallets":     totalWallets,
			"remaining_wallets": totalWallets - i - 1,
		}).Info("Exported wallet token data")
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Total.Cmp(rows[j].Total) > 0
	})

	filename, err := writeNodesTokenCSV(rows)
	if err != nil {
		logger.WithError(err).Error("Failed to write nodes token CSV")
		return
	}

	logger.WithFields(log.Fields{
		"total_wallets": totalWallets,
		"filename":      filename,
	}).Info("Finished exporting nodes token CSV")
}

func buildNodesTokenCSVRow(ctx context.Context, node models.Node, limiter *nodesTokenCSVChainRequestLimiter) (nodesTokenCSVRow, error) {
	dymBalance, err := getNodesTokenCSVChainBalance(ctx, node.Address, nodesTokenCSVDymensionNetwork, limiter)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	nearBalance, err := getNodesTokenCSVChainBalance(ctx, node.Address, nodesTokenCSVNearNetwork, limiter)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	relayAccountBalance, err := service.GetRelayAccountBalance(ctx, config.GetDB(), node.Address)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	staking := new(big.Int).Set(&node.StakeAmount.Int)
	total := new(big.Int).Add(dymBalance, nearBalance)
	total.Add(total, relayAccountBalance)
	total.Add(total, staking)

	return nodesTokenCSVRow{
		Address:             node.Address,
		DymChainBalance:     dymBalance,
		NearChainBalance:    nearBalance,
		RelayAccountBalance: new(big.Int).Set(relayAccountBalance),
		Staking:             staking,
		Total:               total,
	}, nil
}

func getNodesTokenCSVChainBalance(ctx context.Context, address, network string, limiter *nodesTokenCSVChainRequestLimiter) (*big.Int, error) {
	client, err := blockchain.GetBlockchainClient(network)
	if err != nil {
		return nil, err
	}

	balance, err := client.BalanceAt(ctx, common.HexToAddress(address))
	if err != nil {
		return nil, err
	}
	if err := limiter.wait(ctx); err != nil {
		return nil, err
	}
	return balance, nil
}

func (limiter *nodesTokenCSVChainRequestLimiter) wait(ctx context.Context) error {
	limiter.count++
	if limiter.count%nodesTokenCSVChainRequestBatchSize != 0 {
		return nil
	}

	timer := time.NewTimer(nodesTokenCSVChainRequestPause)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writeNodesTokenCSV(rows []nodesTokenCSVRow) (string, error) {
	logDir := getNodesTokenCSVLogDir(config.GetConfig().Log.Output)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}

	filename := filepath.Join(logDir, "nodes_token_"+time.Now().UTC().Format("20060102T150405Z")+".csv")
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{
		"node address",
		"dym chain balance CNX",
		"near chain balance CNX",
		"relay account balance CNX",
		"staking CNX",
		"total CNX",
	}); err != nil {
		return "", err
	}

	for _, row := range rows {
		if err := writer.Write([]string{
			row.Address,
			formatCNXAmount(row.DymChainBalance),
			formatCNXAmount(row.NearChainBalance),
			formatCNXAmount(row.RelayAccountBalance),
			formatCNXAmount(row.Staking),
			formatCNXAmount(row.Total),
		}); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return filename, nil
}

func formatCNXAmount(amount *big.Int) string {
	return utils.WeiToEther(amount).Text('f', 2)
}

func getNodesTokenCSVLogDir(mainLogOutput string) string {
	if mainLogOutput == "" || mainLogOutput == "stdout" || mainLogOutput == "stderr" {
		return "data/logs"
	}
	return filepath.Dir(mainLogOutput)
}

func getAdminLogger() *log.Logger {
	logger := config.GetAdminLogger()
	if logger == nil {
		return log.StandardLogger()
	}
	return logger
}
