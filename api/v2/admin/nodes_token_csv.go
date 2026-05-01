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
	CardName            string
	DymChainBalance     *big.Int
	NearChainBalance    *big.Int
	RelayAccountBalance *big.Int
	Staking             *big.Int
	Total               *big.Int
}

type nodesBenefitAddressCSVRow struct {
	Address        string
	CardName       string
	Network        string
	BenefitAddress string
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

	var networkNodeData []models.NetworkNodeData
	if err := config.GetDB().WithContext(ctx).Model(&models.NetworkNodeData{}).Order("address ASC").Find(&networkNodeData).Error; err != nil {
		logger.WithError(err).Error("Failed to load network node data for token CSV export")
		return
	}

	totalWallets := len(networkNodeData)
	logger.WithField("total_wallets", totalWallets).Info("Loaded wallets from network node data for nodes token CSV export")

	var nodes []models.Node
	if totalWallets > 0 {
		addresses := make([]string, 0, totalWallets)
		for _, nodeData := range networkNodeData {
			addresses = append(addresses, nodeData.Address)
		}
		if err := config.GetDB().WithContext(ctx).Model(&models.Node{}).Where("address IN (?)", addresses).Find(&nodes).Error; err != nil {
			logger.WithError(err).Error("Failed to load nodes for token CSV export")
			return
		}
	}
	nodesByAddress := make(map[string]models.Node, len(nodes))
	for _, node := range nodes {
		nodesByAddress[node.Address] = node
	}

	rows := make([]nodesTokenCSVRow, 0, totalWallets)
	inactiveRows := make([]nodesTokenCSVRow, 0)
	benefitAddressRows := make([]nodesBenefitAddressCSVRow, 0, totalWallets*len(config.GetConfig().Blockchains))
	limiter := &nodesTokenCSVChainRequestLimiter{}
	for i, nodeData := range networkNodeData {
		node, found := nodesByAddress[nodeData.Address]
		staking := &nodeData.Staking.Int
		if found {
			staking = &node.StakeAmount.Int
		}

		row, err := buildNodesTokenCSVRow(ctx, nodeData.Address, nodeData.CardModel, staking, limiter)
		if err != nil {
			logger.WithError(err).WithFields(log.Fields{
				"wallet":            nodeData.Address,
				"total_wallets":     totalWallets,
				"remaining_wallets": totalWallets - i,
			}).Error("Failed to export wallet token data")
			return
		}
		if found {
			rows = append(rows, row)
		} else {
			inactiveRows = append(inactiveRows, row)
		}

		benefitRows, err := buildNodesBenefitAddressCSVRows(ctx, nodeData.Address, nodeData.CardModel, limiter)
		if err != nil {
			logger.WithError(err).WithFields(log.Fields{
				"wallet":            nodeData.Address,
				"total_wallets":     totalWallets,
				"remaining_wallets": totalWallets - i,
			}).Error("Failed to export wallet benefit address data")
			return
		}
		benefitAddressRows = append(benefitAddressRows, benefitRows...)

		logger.WithFields(log.Fields{
			"wallet":            nodeData.Address,
			"total_wallets":     totalWallets,
			"remaining_wallets": totalWallets - i - 1,
		}).Info("Exported wallet token data")
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Total.Cmp(rows[j].Total) > 0
	})
	sort.Slice(inactiveRows, func(i, j int) bool {
		return inactiveRows[i].Total.Cmp(inactiveRows[j].Total) > 0
	})

	filename, err := writeNodesTokenCSV(rows)
	if err != nil {
		logger.WithError(err).Error("Failed to write nodes token CSV")
		return
	}
	inactiveFilename, err := writeInactiveNodesTokenCSV(inactiveRows)
	if err != nil {
		logger.WithError(err).Error("Failed to write inactive nodes token CSV")
		return
	}
	benefitAddressFilename, err := writeNodesBenefitAddressCSV(benefitAddressRows)
	if err != nil {
		logger.WithError(err).Error("Failed to write nodes benefit address CSV")
		return
	}

	logger.WithFields(log.Fields{
		"total_wallets":            totalWallets,
		"active_wallets":           len(rows),
		"inactive_wallets":         len(inactiveRows),
		"filename":                 filename,
		"inactive_filename":        inactiveFilename,
		"benefit_address_filename": benefitAddressFilename,
	}).Info("Finished exporting nodes token CSV")
}

func buildNodesTokenCSVRow(ctx context.Context, address, cardName string, stakingAmount *big.Int, limiter *nodesTokenCSVChainRequestLimiter) (nodesTokenCSVRow, error) {
	dymBalance, err := getNodesTokenCSVChainBalance(ctx, address, nodesTokenCSVDymensionNetwork, limiter)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	nearBalance, err := getNodesTokenCSVChainBalance(ctx, address, nodesTokenCSVNearNetwork, limiter)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	relayAccountBalance, err := service.GetRelayAccountBalance(ctx, config.GetDB(), address)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	staking := new(big.Int).Set(stakingAmount)
	total := new(big.Int).Add(dymBalance, nearBalance)
	total.Add(total, relayAccountBalance)
	total.Add(total, staking)

	return nodesTokenCSVRow{
		Address:             address,
		CardName:            cardName,
		DymChainBalance:     dymBalance,
		NearChainBalance:    nearBalance,
		RelayAccountBalance: new(big.Int).Set(relayAccountBalance),
		Staking:             staking,
		Total:               total,
	}, nil
}

func buildNodesBenefitAddressCSVRows(ctx context.Context, address, cardName string, limiter *nodesTokenCSVChainRequestLimiter) ([]nodesBenefitAddressCSVRow, error) {
	networks := getNodesTokenCSVNetworks()
	rows := make([]nodesBenefitAddressCSVRow, 0, len(networks))
	for _, network := range networks {
		benefitAddress, err := getNodesBenefitAddress(ctx, address, network, limiter)
		if err != nil {
			return nil, err
		}
		rows = append(rows, nodesBenefitAddressCSVRow{
			Address:        address,
			CardName:       cardName,
			Network:        network,
			BenefitAddress: benefitAddress,
		})
	}
	return rows, nil
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

func getNodesBenefitAddress(ctx context.Context, address, network string, limiter *nodesTokenCSVChainRequestLimiter) (string, error) {
	benefitAddress, err := blockchain.GetBenefitAddress(ctx, common.HexToAddress(address), network)
	if err != nil {
		return "", err
	}
	if err := limiter.wait(ctx); err != nil {
		return "", err
	}
	return benefitAddress.Hex(), nil
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
	return writeNodesTokenCSVRows("nodes_token_", rows)
}

func writeInactiveNodesTokenCSV(rows []nodesTokenCSVRow) (string, error) {
	return writeNodesTokenCSVRows("inactive_nodes_token_", rows)
}

func writeNodesTokenCSVRows(filenamePrefix string, rows []nodesTokenCSVRow) (string, error) {
	logDir := getNodesTokenCSVLogDir(config.GetConfig().Log.Output)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}

	filename := filepath.Join(logDir, filenamePrefix+time.Now().UTC().Format("20060102T150405Z")+".csv")
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{
		"node address",
		"card name",
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
			row.CardName,
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

func writeNodesBenefitAddressCSV(rows []nodesBenefitAddressCSVRow) (string, error) {
	logDir := getNodesTokenCSVLogDir(config.GetConfig().Log.Output)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}

	filename := filepath.Join(logDir, "nodes_benefit_addresses_"+time.Now().UTC().Format("20060102T150405Z")+".csv")
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{
		"node address",
		"card name",
		"network",
		"benefit address",
	}); err != nil {
		return "", err
	}

	for _, row := range rows {
		if err := writer.Write([]string{
			row.Address,
			row.CardName,
			row.Network,
			row.BenefitAddress,
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

func getNodesTokenCSVNetworks() []string {
	networks := make([]string, 0, len(config.GetConfig().Blockchains))
	for network := range config.GetConfig().Blockchains {
		networks = append(networks, network)
	}
	sort.Strings(networks)
	return networks
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
