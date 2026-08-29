package config

const (
	EnvProduction = "production"
	EnvDebug      = "debug"
	EnvTest       = "test"
)

const (
	FundingTokenTypeNative = "native"
	FundingTokenTypeERC20  = "erc20"
)

type BlockchainAccountConfig struct {
	Address        string `mapstructure:"address"`
	PrivateKey     string `mapstructure:"private_key"`
	PrivateKeyFile string `mapstructure:"private_key_file"`
}

type SystemBlockchainContractsConfig struct {
	BenefitAddress   string `mapstructure:"benefit_address"`
	NodeStaking      string `mapstructure:"node_staking"`
	DelegatedStaking string `mapstructure:"delegated_staking"`
}

type WithdrawalFeeTierConfig struct {
	MinAmount uint64  `mapstructure:"min_amount" description:"tier lower bound of withdraw amount, in ether unit"`
	FeeRatio  float64 `mapstructure:"fee_ratio" description:"proportional fee ratio applied to the whole withdraw amount in this tier"`
}

type SystemBlockchainConfig struct {
	RPS                            uint64                          `mapstructure:"rps"`
	RpcEndpoint                    string                          `mapstructure:"rpc_endpoint"`
	StartBlockNum                  uint64                          `mapstructure:"start_block_num"`
	GasLimit                       uint64                          `mapstructure:"gas_limit"`
	GasPrice                       uint64                          `mapstructure:"gas_price"`
	ChainID                        uint64                          `mapstructure:"chain_id"`
	Account                        BlockchainAccountConfig         `mapstructure:"account"`
	Contracts                      SystemBlockchainContractsConfig `mapstructure:"contracts"`
	MaxRetries                     uint8                           `mapstructure:"max_retries"`
	RetryInterval                  uint64                          `mapstructure:"retry_interval"`
	SendWaitTime                   uint64                          `mapstructure:"send_wait_time"`
	ReceiptWaitTime                uint64                          `mapstructure:"receipt_wait_time"`
	SentTransactionCountLimit      uint64                          `mapstructure:"sent_transaction_count_limit"`
	DelegatedStakingSlashBatchSize uint64                          `mapstructure:"delegated_staking_slash_batch_size"`
	DelegatedStakingReadPageSize   uint64                          `mapstructure:"delegated_staking_read_page_size"`
	WithdrawalFee                  uint64                          `mapstructure:"withdrawal_fee"`
	WithdrawalMin                  uint64                          `mapstructure:"withdrawal_min"`
	WithdrawalFeeTiers             []WithdrawalFeeTierConfig       `mapstructure:"withdrawal_fee_tiers"`
	MaxWithdrawalsPerDay           uint64                          `mapstructure:"max_withdrawals_per_day"`
}

type DepositWithdrawNetworkContractsConfig struct {
	BenefitAddress string `mapstructure:"benefit_address"`
	TokenAddress   string `mapstructure:"token_address"`
}

type DepositWithdrawNetworkConfig struct {
	RPS                  uint64                                `mapstructure:"rps"`
	RpcEndpoint          string                                `mapstructure:"rpc_endpoint"`
	StartBlockNum        uint64                                `mapstructure:"start_block_num"`
	ChainID              uint64                                `mapstructure:"chain_id"`
	Contracts            DepositWithdrawNetworkContractsConfig `mapstructure:"contracts"`
	LogBlockRange        uint64                                `mapstructure:"log_block_range"`
	WithdrawalFee        uint64                                `mapstructure:"withdrawal_fee"`
	WithdrawalMin        uint64                                `mapstructure:"withdrawal_min"`
	WithdrawalFeeTiers   []WithdrawalFeeTierConfig             `mapstructure:"withdrawal_fee_tiers"`
	MaxWithdrawalsPerDay uint64                                `mapstructure:"max_withdrawals_per_day"`
}

type EffectiveFundingNetworkConfig struct {
	Network              string
	TokenType            string
	RPS                  uint64
	RpcEndpoint          string
	StartBlockNum        uint64
	ChainID              uint64
	BenefitAddress       string
	TokenAddress         string
	LogBlockRange        uint64
	WithdrawalFee        uint64
	WithdrawalMin        uint64
	WithdrawalFeeTiers   []WithdrawalFeeTierConfig
	MaxWithdrawalsPerDay uint64
}

type AppConfig struct {
	Environment string `mapstructure:"environment"`

	Db struct {
		Driver           string `mapstructure:"driver"`
		ConnectionString string `mapstructure:"connection"`
		Log              struct {
			Level       string `mapstructure:"level"`
			Output      string `mapstructure:"output"`
			MaxFileSize int    `mapstructure:"max_file_size"`
			MaxDays     int    `mapstructure:"max_days"`
			MaxFileNum  int    `mapstructure:"max_file_num"`
		} `mapstructure:"log"`
	} `mapstructure:"db"`

	Log struct {
		Level       string `mapstructure:"level"`
		Output      string `mapstructure:"output"`
		MaxFileSize int    `mapstructure:"max_file_size"`
		MaxDays     int    `mapstructure:"max_days"`
		MaxFileNum  int    `mapstructure:"max_file_num"`
		Features    struct {
			NodeHealthEnabled          bool `mapstructure:"node_health_enabled"`
			NodeStatusEnabled          bool `mapstructure:"node_status_enabled"`
			TaskAssignmentEnabled      bool `mapstructure:"task_assignment_enabled"`
			TaskValidationGroupEnabled bool `mapstructure:"task_validation_group_enabled"`
		} `mapstructure:"features"`
	} `mapstructure:"log"`

	Http struct {
		Host         string `mapstructure:"host"`
		Port         string `mapstructure:"port"`
		MaxBodyBytes int64  `mapstructure:"max_body_bytes"`

		JWT struct {
			SecretKey     string `mapstructure:"secret_key"`
			SecretKeyFile string `mapstructure:"secret_key_file"`
			ExpiresIn     uint64 `mapstructure:"expires_in"`
		} `mapstructure:"jwt"`
	} `mapstructure:"http"`

	Admin struct {
		AuthToken            string `mapstructure:"auth_token"`
		VestingSignerAddress string `mapstructure:"vesting_signer_address"`
	} `mapstructure:"admin"`

	DataDir struct {
		InferenceTasks string `mapstructure:"inference_tasks"`
		SlashedTasks   string `mapstructure:"slashed_tasks"`
	} `mapstructure:"data_dir"`

	Blockchains             map[string]SystemBlockchainConfig       `mapstructure:"blockchains"`
	DepositWithdrawNetworks map[string]DepositWithdrawNetworkConfig `mapstructure:"deposit_withdraw_networks"`

	Task struct {
		StakeAmount                uint64 `mapstructure:"stake_amount" description:"stake amount, in ether unit"`
		DistanceThreshold          uint64 `mapstructure:"distance_threshold"`
		TaskWhitelistEnabled       bool   `mapstructure:"task_whitelist_enabled"`
		MinimumNodeNameNumber      uint64 `mapstructure:"minimum_node_name_number"`
		NodeNameWhitelistEnabled   bool   `mapstructure:"node_name_whitelist_enabled"`
		PassiveSlashMode           *bool  `mapstructure:"passive_slash_mode"`
		TaskTracingDurationDays    uint64 `mapstructure:"task_tracing_duration_days"`
		HistoryRetentionDays       uint64 `mapstructure:"history_retention_days"`
		HistoryCleanupBatchSize    int    `mapstructure:"history_cleanup_batch_size"`
		BatchCreateMaxItems        int    `mapstructure:"batch_create_max_items"`
		BatchStatusMaxItems        int    `mapstructure:"batch_status_max_items"`
		BatchValidateMaxItems      int    `mapstructure:"batch_validate_max_items"`
		BatchAbortMaxItems         int    `mapstructure:"batch_abort_max_items"`
		ResultMaxUncompressedBytes int64  `mapstructure:"result_max_uncompressed_bytes"`
		TimeoutQueryBatchSize      int    `mapstructure:"timeout_query_batch_size"`
	} `mapstructure:"task"`

	TaskPricing struct {
		InitialSDOverheadSeconds                  float64 `mapstructure:"initial_sd_overhead_seconds"`
		InitialSecondsPerSDPixelStep              float64 `mapstructure:"initial_seconds_per_sd_pixel_step"`
		InitialLLMConstantSeconds                 float64 `mapstructure:"initial_llm_constant_seconds"`
		InitialLLMSecondsPerInputByte             float64 `mapstructure:"initial_llm_seconds_per_input_byte"`
		InitialLLMSecondsPerOutputToken           float64 `mapstructure:"initial_llm_seconds_per_output_token"`
		InitialLLMModelSwitchSeconds              float64 `mapstructure:"initial_llm_model_switch_seconds"`
		InitialLLMSecondsPerImage                 float64 `mapstructure:"initial_llm_seconds_per_image"`
		InitialLLMSecondsPerMegapixel             float64 `mapstructure:"initial_llm_seconds_per_megapixel"`
		CalibrationAlpha                          float64 `mapstructure:"calibration_alpha"`
		CalibrationRegularization                 float64 `mapstructure:"calibration_regularization"`
		CalibrationMaxPositiveResidualMultiple    float64 `mapstructure:"calibration_max_positive_residual_multiple"`
		CalibrationWarmupSuccessSamples           uint64  `mapstructure:"calibration_warmup_success_samples"`
		CalibrationFlushIntervalSeconds           uint64  `mapstructure:"calibration_flush_interval_seconds"`
		DefaultLLMMaxNewTokens                    uint64  `mapstructure:"default_llm_max_new_tokens"`
		BaseVRAM                                  uint64  `mapstructure:"base_vram"`
		QueueTimeoutSeconds                       uint64  `mapstructure:"queue_timeout_seconds"`
		AppValidationTimeoutSeconds               uint64  `mapstructure:"app_validation_timeout_seconds"`
		ResultUploadTimeoutSeconds                uint64  `mapstructure:"result_upload_timeout_seconds"`
		TimeoutMultiplier                         float64 `mapstructure:"timeout_multiplier"`
		MinExecutionTimeoutSeconds                uint64  `mapstructure:"min_execution_timeout_seconds"`
		MaxExecutionTimeoutSeconds                uint64  `mapstructure:"max_execution_timeout_seconds"`
		QueuedTaskPrioritySnapshotIntervalSeconds uint64  `mapstructure:"queued_task_priority_snapshot_interval_seconds"`
	} `mapstructure:"task_pricing"`

	TaskMatching struct {
		BatchSize           int     `mapstructure:"batch_size"`
		TickIntervalSeconds float64 `mapstructure:"tick_interval_seconds"`
	} `mapstructure:"task_matching"`

	ModelDistribution struct {
		ControllerIntervalSeconds float64 `mapstructure:"controller_interval_seconds"`
		DemandWindowSeconds       float64 `mapstructure:"demand_window_seconds"`
		SafetyFactor              float64 `mapstructure:"safety_factor"`
		MinNodes                  int     `mapstructure:"min_nodes"`
		MaxNodes                  int     `mapstructure:"max_nodes"`
		DownloadTimeoutSeconds    float64 `mapstructure:"download_timeout_seconds"`
	} `mapstructure:"model_distribution"`

	TaskSchema struct {
		StableDiffusionInference    string `mapstructure:"stable_diffusion_inference"`
		GPTInference                string `mapstructure:"gpt_inference"`
		StableDiffusionFinetuneLora string `mapstructure:"stable_diffusion_finetune_lora"`
	} `mapstructure:"task_schema"`

	Stats struct {
		InitStartTime string `mapstructure:"init_start_time"`
	} `mapstructure:"stats"`

	NetworkFLOPS struct {
		GPUFLOPSFile string `mapstructure:"gpu_flops_file"`
	} `mapstructure:"network_flops"`

	Withdraw struct {
		RelayWalletAddress   string `mapstructure:"relay_wallet_address"`
		MinWithdrawalAmount  uint64 `mapstructure:"min_withdrawal_amount"`
		WithdrawalFee        uint64 `mapstructure:"withdrawal_fee"`
		WithdrawalFeeAddress string `mapstructure:"withdrawal_fee_address"`
	} `mapstructure:"withdraw"`

	Dao struct {
		TaskFeeShareAddress string `mapstructure:"task_fee_share_address"`
		TaskFeeSharePercent uint64 `mapstructure:"task_fee_share_percent"`
		MainnetStartTime    string `mapstructure:"mainnet_start_time"`
		AprStartTime        string `mapstructure:"apr_start_time"`
	} `mapstructure:"dao"`

	RelayAccount struct {
		DepositAddress string `mapstructure:"deposit_address"`
	} `mapstructure:"relay_account"`

	MAC struct {
		SecretKey     string `mapstructure:"secret_key"`
		SecretKeyFile string `mapstructure:"secret_key_file"`
	} `mapstructure:"mac"`

	Metrics struct {
		Enabled                     bool     `mapstructure:"enabled"`
		Port                        string   `mapstructure:"port"`
		VramTiers                   []uint64 `mapstructure:"vram_tiers"`
		TaskExecutionTimeoutBuckets []uint64 `mapstructure:"task_execution_timeout_buckets"`
	} `mapstructure:"metrics"`

	StakingScore struct {
		LockedEmissionCoefficient *float64 `mapstructure:"locked_emission_coefficient" description:"coefficient applied to locked emission before it is counted into the staking score"`
	} `mapstructure:"staking_score"`

	QoS struct {
		ScorePoolSize               uint64  `mapstructure:"score_pool_size"`
		TracingMaxTaskEvents        uint64  `mapstructure:"tracing_max_task_events"`
		KickoutThreshold            float64 `mapstructure:"kickout_threshold"`
		RejoinQosLongFloor          float64 `mapstructure:"rejoin_qos_long_floor"`
		PenaltyFactor               float64 `mapstructure:"penalty_factor"`
		FirstTimeoutPenaltyFactor   float64 `mapstructure:"first_timeout_penalty_factor"`
		FirstTimeoutHealthThreshold float64 `mapstructure:"first_timeout_health_threshold"`
		SuccessBoost                float64 `mapstructure:"success_boost"`
		RecoveryTauMinutes          float64 `mapstructure:"recovery_tau_minutes"`
		HealthExcludeEnterThreshold float64 `mapstructure:"health_exclude_enter_threshold"`
		HealthExcludeExitThreshold  float64 `mapstructure:"health_exclude_exit_threshold"`
	} `mapstructure:"qos"`
}
