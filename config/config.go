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
	if err := checkNetworkFLOPSConfig(); err != nil {
		return err
	}
	if err := checkTaskConfig(); err != nil {
		return err
	}
	if err := checkTaskPricingConfig(); err != nil {
		return err
	}
	if err := checkTaskMatchingConfig(); err != nil {
		return err
	}
	if err := checkModelDistributionConfig(); err != nil {
		return err
	}
	if err := checkStakingScoreConfig(); err != nil {
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
		if blockchain.Contracts.DelegatedStaking != "" && !common.IsHexAddress(blockchain.Contracts.DelegatedStaking) {
			return fmt.Errorf("blockchain %s delegated staking contract is invalid", network)
		}
	}
	for network, blockchain := range appConfig.Blockchains {
		if blockchain.MaxWithdrawalsPerDay == 0 {
			return fmt.Errorf("blockchain %s max_withdrawals_per_day is not set", network)
		}
		if err := checkWithdrawalFeeTiers(network, blockchain.WithdrawalFeeTiers); err != nil {
			return err
		}
	}
	for network, fundingNetwork := range appConfig.DepositWithdrawNetworks {
		if fundingNetwork.MaxWithdrawalsPerDay == 0 {
			return fmt.Errorf("deposit withdraw network %s max_withdrawals_per_day is not set", network)
		}
		if err := checkWithdrawalFeeTiers(network, fundingNetwork.WithdrawalFeeTiers); err != nil {
			return err
		}
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

func checkNetworkFLOPSConfig() error {
	if strings.TrimSpace(appConfig.NetworkFLOPS.GPUFLOPSFile) == "" {
		return errors.New("network_flops.gpu_flops_file is not set")
	}
	return nil
}

func checkTaskConfig() error {
	if appConfig.Task.PassiveSlashMode == nil {
		return errors.New("task.passive_slash_mode is not set")
	}
	if appConfig.Task.HistoryCleanupBatchSize <= 0 {
		return errors.New("task.history_cleanup_batch_size is not set")
	}
	if appConfig.Task.BatchCreateMaxItems <= 0 {
		return errors.New("task.batch_create_max_items is not set")
	}
	if appConfig.Task.BatchStatusMaxItems <= 0 {
		return errors.New("task.batch_status_max_items is not set")
	}
	if appConfig.Task.BatchValidateMaxItems <= 0 {
		return errors.New("task.batch_validate_max_items is not set")
	}
	if appConfig.Task.BatchAbortMaxItems <= 0 {
		return errors.New("task.batch_abort_max_items is not set")
	}
	if appConfig.Task.ResultMaxUncompressedBytes <= 0 {
		return errors.New("task.result_max_uncompressed_bytes is not set")
	}
	if appConfig.Task.TimeoutQueryBatchSize <= 0 {
		return errors.New("task.timeout_query_batch_size is not set")
	}
	return nil
}

func checkTaskPricingConfig() error {
	pricing := appConfig.TaskPricing
	if pricing.InitialSDOverheadSeconds <= 0 {
		return errors.New("task_pricing.initial_sd_overhead_seconds is not set")
	}
	if pricing.InitialSecondsPerSDPixelStep <= 0 {
		return errors.New("task_pricing.initial_seconds_per_sd_pixel_step is not set")
	}
	if pricing.InitialLLMConstantSeconds <= 0 {
		return errors.New("task_pricing.initial_llm_constant_seconds is not set")
	}
	if pricing.InitialLLMSecondsPerInputByte <= 0 {
		return errors.New("task_pricing.initial_llm_seconds_per_input_byte is not set")
	}
	if pricing.InitialLLMSecondsPerOutputToken <= 0 {
		return errors.New("task_pricing.initial_llm_seconds_per_output_token is not set")
	}
	if pricing.InitialLLMModelSwitchSeconds <= 0 {
		return errors.New("task_pricing.initial_llm_model_switch_seconds is not set")
	}
	if pricing.InitialLLMSecondsPerImage <= 0 {
		return errors.New("task_pricing.initial_llm_seconds_per_image is not set")
	}
	if pricing.InitialLLMSecondsPerMegapixel <= 0 {
		return errors.New("task_pricing.initial_llm_seconds_per_megapixel is not set")
	}
	if pricing.CalibrationAlpha <= 0 || pricing.CalibrationAlpha >= 1 {
		return errors.New("task_pricing.calibration_alpha must be in (0, 1)")
	}
	if pricing.CalibrationRegularization <= 0 {
		return errors.New("task_pricing.calibration_regularization is not set")
	}
	if pricing.CalibrationMaxPositiveResidualMultiple <= 0 {
		return errors.New("task_pricing.calibration_max_positive_residual_multiple is not set")
	}
	if pricing.CalibrationWarmupSuccessSamples < 3 {
		return errors.New("task_pricing.calibration_warmup_success_samples must be at least 3")
	}
	if pricing.CalibrationFlushIntervalSeconds == 0 {
		return errors.New("task_pricing.calibration_flush_interval_seconds is not set")
	}
	if pricing.DefaultLLMMaxNewTokens == 0 {
		return errors.New("task_pricing.default_llm_max_new_tokens is not set")
	}
	if pricing.BaseVRAM == 0 {
		return errors.New("task_pricing.base_vram is not set")
	}
	if pricing.QueueTimeoutSeconds == 0 {
		return errors.New("task_pricing.queue_timeout_seconds is not set")
	}
	if pricing.AppValidationTimeoutSeconds == 0 {
		return errors.New("task_pricing.app_validation_timeout_seconds is not set")
	}
	if pricing.ResultUploadTimeoutSeconds == 0 {
		return errors.New("task_pricing.result_upload_timeout_seconds is not set")
	}
	if pricing.TimeoutMultiplier <= 0 {
		return errors.New("task_pricing.timeout_multiplier is not set")
	}
	if pricing.MinExecutionTimeoutSeconds == 0 {
		return errors.New("task_pricing.min_execution_timeout_seconds is not set")
	}
	if pricing.MaxExecutionTimeoutSeconds < pricing.MinExecutionTimeoutSeconds {
		return errors.New("task_pricing.max_execution_timeout_seconds must be at least min_execution_timeout_seconds")
	}
	if pricing.QueuedTaskPrioritySnapshotIntervalSeconds == 0 {
		return errors.New("task_pricing.queued_task_priority_snapshot_interval_seconds is not set")
	}
	return nil
}

func checkTaskMatchingConfig() error {
	matching := appConfig.TaskMatching
	if matching.BatchSize <= 0 {
		return errors.New("task_matching.batch_size is not set")
	}
	if matching.TickIntervalSeconds <= 0 {
		return errors.New("task_matching.tick_interval_seconds is not set")
	}
	return nil
}

func checkModelDistributionConfig() error {
	distribution := appConfig.ModelDistribution
	if distribution.ControllerIntervalSeconds <= 0 {
		return errors.New("model_distribution.controller_interval_seconds is not set")
	}
	if distribution.DemandWindowSeconds <= 0 {
		return errors.New("model_distribution.demand_window_seconds is not set")
	}
	if distribution.SafetyFactor <= 0 {
		return errors.New("model_distribution.safety_factor is not set")
	}
	if distribution.MinNodes <= 0 {
		return errors.New("model_distribution.min_nodes is not set")
	}
	if distribution.MaxNodes < distribution.MinNodes {
		return errors.New("model_distribution.max_nodes must be at least model_distribution.min_nodes")
	}
	if distribution.DownloadTimeoutSeconds <= 0 {
		return errors.New("model_distribution.download_timeout_seconds is not set")
	}
	return nil
}

func checkWithdrawalFeeTiers(network string, tiers []WithdrawalFeeTierConfig) error {
	if len(tiers) == 0 {
		return nil
	}
	if tiers[0].MinAmount != 0 {
		return fmt.Errorf("network %s withdrawal_fee_tiers first tier min_amount must be 0", network)
	}
	for i, tier := range tiers {
		if i > 0 && tier.MinAmount <= tiers[i-1].MinAmount {
			return fmt.Errorf("network %s withdrawal_fee_tiers min_amount must be strictly increasing", network)
		}
		if tier.FeeRatio < 0 || tier.FeeRatio >= 1 {
			return fmt.Errorf("network %s withdrawal_fee_tiers fee_ratio must be in [0, 1)", network)
		}
	}
	return nil
}

func checkStakingScoreConfig() error {
	coefficient := appConfig.StakingScore.LockedEmissionCoefficient
	if coefficient == nil {
		return errors.New("staking_score.locked_emission_coefficient is not set")
	}
	if *coefficient < 0 || *coefficient > 1 {
		return errors.New("staking_score.locked_emission_coefficient must be in [0, 1]")
	}
	return nil
}

func checkQosConfig() error {
	if appConfig.QoS.TracingMaxTaskEvents == 0 {
		return errors.New("qos.tracing_max_task_events is not set")
	}
	if appConfig.QoS.HealthExcludeEnterThreshold <= 0 ||
		appConfig.QoS.HealthExcludeEnterThreshold >= appConfig.QoS.HealthExcludeExitThreshold ||
		appConfig.QoS.HealthExcludeExitThreshold > 1 {
		return errors.New("qos health exclusion thresholds must satisfy 0 < health_exclude_enter_threshold < health_exclude_exit_threshold <= 1")
	}
	if appConfig.QoS.PenaltyFactor <= 0 || appConfig.QoS.PenaltyFactor > 1 {
		return errors.New("qos.penalty_factor must be in (0, 1]")
	}
	if appConfig.QoS.FirstTimeoutPenaltyFactor <= 0 || appConfig.QoS.FirstTimeoutPenaltyFactor > 1 {
		return errors.New("qos.first_timeout_penalty_factor must be in (0, 1]")
	}
	if appConfig.QoS.FirstTimeoutHealthThreshold <= 0 || appConfig.QoS.FirstTimeoutHealthThreshold > 1 {
		return errors.New("qos.first_timeout_health_threshold must be in (0, 1]")
	}
	if appConfig.QoS.SuccessBoost < 0 || appConfig.QoS.SuccessBoost > 1 {
		return errors.New("qos.success_boost must be in [0, 1]")
	}
	if appConfig.QoS.RecoveryTauMinutes <= 0 {
		return errors.New("qos.recovery_tau_minutes must be greater than 0")
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
	if len(appConfig.Metrics.TaskExecutionTimeoutBuckets) == 0 {
		return errors.New("metrics.task_execution_timeout_buckets is not set")
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
