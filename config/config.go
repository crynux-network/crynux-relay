package config

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
)

var appConfig *AppConfig

// InitConfig Init is an exported method that takes the config from the config file
// and unmarshal it into AppConfig struct
func InitConfig(configPath string) error {
	v := viper.New()
	v.SetConfigType("yml")
	v.SetConfigName("config")

	if configPath != "" {
		v.AddConfigPath(configPath)
	} else {
		v.AddConfigPath("/app/config")
		v.AddConfigPath("config")
	}

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	appConfig = &AppConfig{}

	if err := v.Unmarshal(appConfig); err != nil {
		return err
	}

	if appConfig.Environment == EnvTest {
		privKey := GetTestPrivateKey()
		for network := range appConfig.Blockchains {
			blockchain := appConfig.Blockchains[network]
			blockchain.Account.PrivateKey = privKey
			appConfig.Blockchains[network] = blockchain
		}
		appConfig.Http.JWT.SecretKey = GetTestJWTKey()
		appConfig.MAC.SecretKey = GetTestTaskFeeMACKey()
	} else {
		// Load hard-coded private key
		for network := range appConfig.Blockchains {
			blockchain := appConfig.Blockchains[network]
			blockchain.Account.PrivateKey = ReadFromFile(blockchain.Account.PrivateKeyFile)
			appConfig.Blockchains[network] = blockchain
		}
		appConfig.Http.JWT.SecretKey = ReadFromFile(appConfig.Http.JWT.SecretKeyFile)
		appConfig.MAC.SecretKey = ReadFromFile(appConfig.MAC.SecretKeyFile)
	}
	if err := checkBlockchainAccount(); err != nil {
		return err
	}
	if err := checkFundingNetworks(); err != nil {
		return err
	}
	if err := checkHttpConfig(); err != nil {
		return err
	}
	if err := checkStatsConfig(); err != nil {
		return err
	}
	if err := checkTaskConfig(); err != nil {
		return err
	}
	if err := checkQosConfig(); err != nil {
		return err
	}
	if err := checkDaoConfig(); err != nil {
		return err
	}
	if err := checkMetricsConfig(); err != nil {
		return err
	}

	return nil
}

func checkBlockchainAccount() error {

	for network, blockchain := range appConfig.Blockchains {
		blockchain.Account.PrivateKey = NormalizePrivateKey(blockchain.Account.PrivateKey)
		appConfig.Blockchains[network] = blockchain

		if blockchain.Account.PrivateKey == "" {
			return errors.New("blockchain account private key not set")
		}

		if blockchain.Account.Address == "" {
			return errors.New("blockchain account address not set")
		}

		// Check private key and address
		privateKey, err := crypto.HexToECDSA(blockchain.Account.PrivateKey)
		if err != nil {
			return err
		}

		publicKey := privateKey.Public()

		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("error casting public key to ECDSA")
		}

		address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

		if address != blockchain.Account.Address {
			return errors.New("account address and private key mismatch")
		}
	}

	return nil
}

func checkFundingNetworks() error {
	if _, err := appConfig.AllBlockchainNetworks(); err != nil {
		return err
	}
	for network, blockchain := range appConfig.Blockchains {
		if blockchain.RPS == 0 {
			return fmt.Errorf("blockchain %s rps not set", network)
		}
		if blockchain.RpcEndpoint == "" {
			return fmt.Errorf("blockchain %s rpc endpoint not set", network)
		}
		if !common.IsHexAddress(blockchain.Contracts.BenefitAddress) {
			return fmt.Errorf("blockchain %s benefit address contract is invalid", network)
		}
		if !common.IsHexAddress(blockchain.Contracts.NodeStaking) {
			return fmt.Errorf("blockchain %s node staking contract is invalid", network)
		}
		if !common.IsHexAddress(blockchain.Contracts.Credits) {
			return fmt.Errorf("blockchain %s credits contract is invalid", network)
		}
		if blockchain.Contracts.DelegatedStaking != "" && !common.IsHexAddress(blockchain.Contracts.DelegatedStaking) {
			return fmt.Errorf("blockchain %s delegated staking contract is invalid", network)
		}
	}
	for network, fundingNetwork := range appConfig.DepositWithdrawNetworks {
		if fundingNetwork.RPS == 0 {
			return fmt.Errorf("deposit withdraw network %s rps not set", network)
		}
		if fundingNetwork.RpcEndpoint == "" {
			return fmt.Errorf("deposit withdraw network %s rpc endpoint not set", network)
		}
		if !common.IsHexAddress(fundingNetwork.Contracts.BenefitAddress) {
			return fmt.Errorf("deposit withdraw network %s benefit address contract is invalid", network)
		}
		if !common.IsHexAddress(fundingNetwork.Contracts.TokenAddress) {
			return fmt.Errorf("deposit withdraw network %s token address is invalid", network)
		}
		if fundingNetwork.LogBlockRange == 0 {
			return fmt.Errorf("deposit withdraw network %s log block range not set", network)
		}
	}
	return nil
}

func checkHttpConfig() error {
	if appConfig.Http.MaxBodyBytes <= 0 {
		return errors.New("http.max_body_bytes is not set")
	}
	return nil
}

func checkStatsConfig() error {
	raw := strings.TrimSpace(appConfig.Stats.InitStartTime)
	if raw == "" {
		return errors.New("stats.init_start_time is not set")
	}
	if _, err := time.Parse(time.RFC3339, raw); err != nil {
		return fmt.Errorf("stats.init_start_time must be RFC3339: %w", err)
	}
	appConfig.Stats.InitStartTime = raw
	return nil
}

func checkTaskConfig() error {
	if appConfig.Task.PassiveSlashMode == nil {
		return errors.New("task.passive_slash_mode is not set")
	}
	return nil
}

func checkQosConfig() error {
	if appConfig.QoS.TracingMaxTaskEvents == 0 {
		return errors.New("qos.tracing_max_task_events is not set")
	}
	return nil
}

func checkDaoConfig() error {
	rawAPRStartTime := strings.TrimSpace(appConfig.Dao.AprStartTime)
	if rawAPRStartTime == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339, rawAPRStartTime); err != nil {
		return fmt.Errorf("dao.apr_start_time must be RFC3339: %w", err)
	}
	appConfig.Dao.AprStartTime = rawAPRStartTime
	return nil
}

func checkMetricsConfig() error {
	if !appConfig.Metrics.Enabled {
		return nil
	}
	if strings.TrimSpace(appConfig.Metrics.Port) == "" {
		return errors.New("metrics.port is not set")
	}
	if len(appConfig.Metrics.VramTiers) == 0 {
		return errors.New("metrics.vram_tiers is not set")
	}
	return nil
}

func NormalizePrivateKey(privateKey string) string {
	privateKey = strings.TrimSpace(privateKey)
	if len(privateKey) >= 2 && strings.EqualFold(privateKey[:2], "0x") {
		return privateKey[2:]
	}
	return privateKey
}

func ReadFromFile(file string) string {
	b, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(b))
}

func DeleteBlockchainPrivateKeyFilesAfterRead() error {
	if appConfig == nil {
		return nil
	}

	var files []string
	for _, blockchain := range appConfig.Blockchains {
		files = append(files, blockchain.Account.PrivateKeyFile)
	}

	deletedFiles := make(map[string]struct{}, len(files))
	for _, file := range files {
		if _, ok := deletedFiles[file]; ok {
			continue
		}
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("delete blockchain private key file %s: %w", file, err)
		}
		deletedFiles[file] = struct{}{}
	}
	return nil
}

func GetTestPrivateKey() string {
	return ""
}

func GetTestJWTKey() string {
	return ""
}

func GetTestTaskFeeMACKey() string {
	return ""
}

func GetConfig() *AppConfig {
	return appConfig
}
