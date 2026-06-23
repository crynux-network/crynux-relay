package admin

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"crynux_relay/utils"
	"encoding/csv"
	"fmt"
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
	nodesTokenCSVChainRequestMaxRetry  = 10
)

var nodesTokenCSVChainRequestRetryWait = 10 * time.Second

var nodesTokenCSVExportState = struct {
	sync.Mutex
	running bool
}{}

type nodesTokenCSVRow struct {
	Address                string
	CardName               string
	ChainBalances          map[string]*big.Int
	BenefitAddressBalances map[string]*big.Int
	RelayAccountBalance    *big.Int
	Staking                *big.Int
}

type nodesBenefitAddressCSVRow struct {
	Address               string
	CardName              string
	Network               string
	BenefitAddress        string
	BenefitAddressBalance *big.Int
}

type nodesActiveDelegatedStakingCSVRow struct {
	DelegatorAddress    string
	NodeAddress         string
	Network             string
	ChainStakingBalance *big.Int
	ChainWalletBalance  *big.Int
}

type nodesActiveDelegatedStakingNode struct {
	NodeAddress string
	Network     string
}

type nodesActiveDelegatedStakingInfos struct {
	DelegatorAddresses []common.Address
	Amounts            []*big.Int
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

	networks := getNodesTokenCSVNetworks()
	rows := make([]nodesTokenCSVRow, 0, totalWallets)
	benefitAddressRows := make([]nodesBenefitAddressCSVRow, 0, totalWallets*len(networks))
	limiter := &nodesTokenCSVChainRequestLimiter{}
	for i, nodeData := range networkNodeData {
		staking := &nodeData.Staking.Int

		benefitRows, err := buildNodesBenefitAddressCSVRows(ctx, nodeData.Address, nodeData.CardModel, networks, limiter)
		if err != nil {
			logger.WithError(err).WithFields(log.Fields{
				"wallet":            nodeData.Address,
				"total_wallets":     totalWallets,
				"remaining_wallets": totalWallets - i,
			}).Error("Failed to export wallet benefit address data")
			return
		}
		benefitAddressRows = append(benefitAddressRows, benefitRows...)

		row, err := buildNodesTokenCSVRow(ctx, nodeData.Address, nodeData.CardModel, staking, networks, benefitRows, limiter)
		if err != nil {
			logger.WithError(err).WithFields(log.Fields{
				"wallet":            nodeData.Address,
				"total_wallets":     totalWallets,
				"remaining_wallets": totalWallets - i,
			}).Error("Failed to export wallet token data")
			return
		}
		rows = append(rows, row)

		logger.WithFields(log.Fields{
			"wallet":            nodeData.Address,
			"total_wallets":     totalWallets,
			"remaining_wallets": totalWallets - i - 1,
		}).Info("Exported wallet token data")
	}

	filename, err := writeNodesTokenCSV(rows, networks)
	if err != nil {
		logger.WithError(err).Error("Failed to write nodes token CSV")
		return
	}
	benefitAddressFilename, err := writeNodesBenefitAddressCSV(benefitAddressRows)
	if err != nil {
		logger.WithError(err).Error("Failed to write nodes benefit address CSV")
		return
	}
	activeDelegatedStakingRows, err := buildNodesActiveDelegatedStakingCSVRows(ctx, limiter)
	if err != nil {
		logger.WithError(err).Error("Failed to export active delegated staking token data")
		return
	}
	activeDelegatedStakingFilename, err := writeNodesActiveDelegatedStakingCSV(activeDelegatedStakingRows)
	if err != nil {
		logger.WithError(err).Error("Failed to write active delegated staking CSV")
		return
	}

	logger.WithFields(log.Fields{
		"total_wallets":                          totalWallets,
		"wallets":                                len(rows),
		"active_delegated_staking_rows":          len(activeDelegatedStakingRows),
		"filename":                               filename,
		"benefit_address_filename":               benefitAddressFilename,
		"active_delegated_staking_rows_filename": activeDelegatedStakingFilename,
	}).Info("Finished exporting nodes token CSV")
}

func buildNodesTokenCSVRow(ctx context.Context, address, cardName string, stakingAmount *big.Int, networks []string, benefitRows []nodesBenefitAddressCSVRow, limiter *nodesTokenCSVChainRequestLimiter) (nodesTokenCSVRow, error) {
	chainBalances := make(map[string]*big.Int, len(networks))
	benefitAddressBalances := make(map[string]*big.Int, len(benefitRows))
	for _, network := range networks {
		balance, err := getNodesTokenCSVChainBalance(ctx, address, network, limiter)
		if err != nil {
			return nodesTokenCSVRow{}, err
		}
		chainBalances[network] = balance
	}
	for _, benefitRow := range benefitRows {
		balance := new(big.Int).Set(benefitRow.BenefitAddressBalance)
		benefitAddressBalances[benefitRow.Network] = balance
	}
	relayAccountBalance, err := service.GetRelayAccountBalance(ctx, config.GetDB(), address)
	if err != nil {
		return nodesTokenCSVRow{}, err
	}
	staking := new(big.Int).Set(stakingAmount)

	return nodesTokenCSVRow{
		Address:                address,
		CardName:               cardName,
		ChainBalances:          chainBalances,
		BenefitAddressBalances: benefitAddressBalances,
		RelayAccountBalance:    new(big.Int).Set(relayAccountBalance),
		Staking:                staking,
	}, nil
}

func buildNodesBenefitAddressCSVRows(ctx context.Context, address, cardName string, networks []string, limiter *nodesTokenCSVChainRequestLimiter) ([]nodesBenefitAddressCSVRow, error) {
	rows := make([]nodesBenefitAddressCSVRow, 0, len(networks))
	nodeAddress := common.HexToAddress(address)
	for _, network := range networks {
		benefitAddress, err := getNodesBenefitAddress(ctx, address, network, limiter)
		if err != nil {
			return nil, err
		}
		if nodeAddress == common.HexToAddress(benefitAddress) {
			continue
		}
		benefitAddressBalance, err := getNodesTokenCSVChainBalance(ctx, benefitAddress, network, limiter)
		if err != nil {
			return nil, err
		}
		rows = append(rows, nodesBenefitAddressCSVRow{
			Address:               address,
			CardName:              cardName,
			Network:               network,
			BenefitAddress:        benefitAddress,
			BenefitAddressBalance: benefitAddressBalance,
		})
	}
	return rows, nil
}

func getNodesTokenCSVChainBalance(ctx context.Context, address, network string, limiter *nodesTokenCSVChainRequestLimiter) (*big.Int, error) {
	client, err := blockchain.GetBlockchainClient(network)
	if err != nil {
		return nil, err
	}

	balance, err := retryNodesTokenCSVChainRequest(ctx, "balance", network, address, func() (*big.Int, error) {
		return client.BalanceAt(ctx, common.HexToAddress(address))
	})
	if err != nil {
		return nil, err
	}
	if err := limiter.wait(ctx); err != nil {
		return nil, err
	}
	return balance, nil
}

func getNodesBenefitAddress(ctx context.Context, address, network string, limiter *nodesTokenCSVChainRequestLimiter) (string, error) {
	benefitAddress, err := retryNodesTokenCSVChainRequest(ctx, "benefit_address", network, address, func() (common.Address, error) {
		return blockchain.GetBenefitAddress(ctx, common.HexToAddress(address), network)
	})
	if err != nil {
		return "", err
	}
	if err := limiter.wait(ctx); err != nil {
		return "", err
	}
	return benefitAddress.Hex(), nil
}

func buildNodesActiveDelegatedStakingCSVRows(ctx context.Context, limiter *nodesTokenCSVChainRequestLimiter) ([]nodesActiveDelegatedStakingCSVRow, error) {
	nodes, err := loadNodesActiveDelegatedStakingNodes(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]nodesActiveDelegatedStakingCSVRow, 0)
	for _, node := range nodes {
		stakingInfos, err := getNodesActiveDelegatedStakingInfos(ctx, node.NodeAddress, node.Network, limiter)
		if err != nil {
			return nil, err
		}
		if len(stakingInfos.DelegatorAddresses) != len(stakingInfos.Amounts) {
			return nil, fmt.Errorf("delegated staking info length mismatch for node %s on network %s: %d addresses, %d amounts", node.NodeAddress, node.Network, len(stakingInfos.DelegatorAddresses), len(stakingInfos.Amounts))
		}
		for i, delegatorAddress := range stakingInfos.DelegatorAddresses {
			amount := stakingInfos.Amounts[i]
			if amount.Sign() == 0 {
				continue
			}
			walletBalance, err := getNodesTokenCSVChainBalance(ctx, delegatorAddress.Hex(), node.Network, limiter)
			if err != nil {
				return nil, err
			}
			rows = append(rows, nodesActiveDelegatedStakingCSVRow{
				DelegatorAddress:    delegatorAddress.Hex(),
				NodeAddress:         node.NodeAddress,
				Network:             node.Network,
				ChainStakingBalance: new(big.Int).Set(amount),
				ChainWalletBalance:  walletBalance,
			})
		}
	}
	return rows, nil
}

func loadNodesActiveDelegatedStakingNodes(ctx context.Context) ([]nodesActiveDelegatedStakingNode, error) {
	var nodes []nodesActiveDelegatedStakingNode
	err := config.GetDB().WithContext(ctx).
		Model(&models.Delegation{}).
		Select("delegations.node_address, delegations.network").
		Joins("inner join nodes on nodes.address = delegations.node_address and nodes.network = delegations.network").
		Where("delegations.slashed = ?", false).
		Group("delegations.node_address, delegations.network").
		Order("delegations.node_address ASC, delegations.network ASC").
		Find(&nodes).Error
	return nodes, err
}

func getNodesActiveDelegatedStakingInfos(ctx context.Context, nodeAddress, network string, limiter *nodesTokenCSVChainRequestLimiter) (nodesActiveDelegatedStakingInfos, error) {
	infos, err := retryNodesTokenCSVChainRequest(ctx, "delegated_staking_infos", network, nodeAddress, func() (nodesActiveDelegatedStakingInfos, error) {
		addresses, amounts, err := blockchain.GetNodeStakingInfos(ctx, common.HexToAddress(nodeAddress), network)
		if err != nil {
			return nodesActiveDelegatedStakingInfos{}, err
		}
		return nodesActiveDelegatedStakingInfos{
			DelegatorAddresses: addresses,
			Amounts:            amounts,
		}, nil
	})
	if err != nil {
		return nodesActiveDelegatedStakingInfos{}, err
	}
	if err := limiter.wait(ctx); err != nil {
		return nodesActiveDelegatedStakingInfos{}, err
	}
	return infos, nil
}

func retryNodesTokenCSVChainRequest[T any](ctx context.Context, operation, network, address string, request func() (T, error)) (T, error) {
	logger := getAdminLogger()
	var zero T
	var lastErr error

	for retry := 0; retry <= nodesTokenCSVChainRequestMaxRetry; retry++ {
		result, err := request()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		if retry == nodesTokenCSVChainRequestMaxRetry {
			break
		}

		logger.WithError(err).WithFields(log.Fields{
			"operation":   operation,
			"network":     network,
			"address":     address,
			"retry":       retry + 1,
			"max_retries": nodesTokenCSVChainRequestMaxRetry,
			"retry_after": nodesTokenCSVChainRequestRetryWait.String(),
		}).Warn("Retrying nodes token CSV blockchain request")

		timer := time.NewTimer(nodesTokenCSVChainRequestRetryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}

	return zero, fmt.Errorf("nodes token CSV blockchain request failed after %d retries: %w", nodesTokenCSVChainRequestMaxRetry, lastErr)
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

func writeNodesTokenCSV(rows []nodesTokenCSVRow, networks []string) (string, error) {
	return writeNodesTokenCSVRows("nodes_token_", rows, networks)
}

func writeNodesTokenCSVRows(filenamePrefix string, rows []nodesTokenCSVRow, networks []string) (string, error) {
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
	if err := writer.Write(buildNodesTokenCSVHeader(networks)); err != nil {
		return "", err
	}

	for _, row := range rows {
		if err := writer.Write(buildNodesTokenCSVRecord(row, networks)); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return filename, nil
}

func buildNodesTokenCSVHeader(networks []string) []string {
	header := []string{
		"node address",
		"card name",
	}
	for _, network := range networks {
		header = append(header, fmt.Sprintf("%s node on-chain wallet balance CNX", network))
	}
	header = append(header,
		"staking CNX",
		"relay account balance CNX",
	)
	for _, network := range networks {
		header = append(header, fmt.Sprintf("%s benefit address balance CNX", network))
	}
	return header
}

func buildNodesTokenCSVRecord(row nodesTokenCSVRow, networks []string) []string {
	record := []string{
		row.Address,
		row.CardName,
	}
	for _, network := range networks {
		record = append(record, formatCNXAmount(getNodesTokenCSVNetworkAmount(row.ChainBalances, network)))
	}
	record = append(record,
		formatCNXAmount(row.Staking),
		formatCNXAmount(row.RelayAccountBalance),
	)
	for _, network := range networks {
		record = append(record, formatCNXAmount(getNodesTokenCSVNetworkAmount(row.BenefitAddressBalances, network)))
	}
	return record
}

func getNodesTokenCSVNetworkAmount(amounts map[string]*big.Int, network string) *big.Int {
	amount, ok := amounts[network]
	if !ok {
		return big.NewInt(0)
	}
	return amount
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
		"benefit address balance CNX",
	}); err != nil {
		return "", err
	}

	for _, row := range rows {
		if err := writer.Write([]string{
			row.Address,
			row.CardName,
			row.Network,
			row.BenefitAddress,
			formatCNXAmount(row.BenefitAddressBalance),
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

func writeNodesActiveDelegatedStakingCSV(rows []nodesActiveDelegatedStakingCSVRow) (string, error) {
	logDir := getNodesTokenCSVLogDir(config.GetConfig().Log.Output)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}

	filename := filepath.Join(logDir, "active_delegated_staking_"+time.Now().UTC().Format("20060102T150405Z")+".csv")
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(buildNodesActiveDelegatedStakingCSVHeader()); err != nil {
		return "", err
	}

	for _, row := range rows {
		if err := writer.Write(buildNodesActiveDelegatedStakingCSVRecord(row)); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return filename, nil
}

func buildNodesActiveDelegatedStakingCSVHeader() []string {
	return []string{
		"delegator address",
		"node address",
		"network",
		"chain staking balance CNX",
		"chain wallet balance CNX",
	}
}

func buildNodesActiveDelegatedStakingCSVRecord(row nodesActiveDelegatedStakingCSVRow) []string {
	return []string{
		row.DelegatorAddress,
		row.NodeAddress,
		row.Network,
		formatCNXAmount(row.ChainStakingBalance),
		formatCNXAmount(row.ChainWalletBalance),
	}
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
