package config

const (
	EnvProduction = "production"
	EnvDebug      = "debug"
	EnvTest       = "test"
)

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
		Host string `mapstructure:"host"`
		Port string `mapstructure:"port"`

		JWT struct {
			SecretKey     string `mapstructure:"secret_key"`
			SecretKeyFile string `mapstructure:"secret_key_file"`
			ExpiresIn     uint64 `mapstructure:"expires_in"`
		} `mapstructure:"jwt"`
	} `mapstructure:"http"`

	Admin struct {
		AuthToken string `mapstructure:"auth_token"`
	} `mapstructure:"admin"`

	DataDir struct {
		InferenceTasks string `mapstructure:"inference_tasks"`
		SlashedTasks   string `mapstructure:"slashed_tasks"`
	} `mapstructure:"data_dir"`

	Blockchains map[string]struct {
		RPS           uint64 `mapstructure:"rps"`
		RpcEndpoint   string `mapstructure:"rpc_endpoint"`
		StartBlockNum uint64 `mapstructure:"start_block_num"`
		GasLimit      uint64 `mapstructure:"gas_limit"`
		GasPrice      uint64 `mapstructure:"gas_price"`
		ChainID       uint64 `mapstructure:"chain_id"`
		Account       struct {
			Address        string `mapstructure:"address"`
			PrivateKey     string `mapstructure:"private_key"`
			PrivateKeyFile string `mapstructure:"private_key_file"`
		} `mapstructure:"account"`
		Contracts struct {
			BenefitAddress   string `mapstructure:"benefit_address"`
			NodeStaking      string `mapstructure:"node_staking"`
			Credits          string `mapstructure:"credits"`
			DelegatedStaking string `mapstructure:"delegated_staking"`
		} `mapstructure:"contracts"`
		MaxRetries                uint8  `mapstructure:"max_retries"`
		RetryInterval             uint64 `mapstructure:"retry_interval"`
		SendWaitTime              uint64 `mapstructure:"send_wait_time"`
		ReceiptWaitTime           uint64 `mapstructure:"receipt_wait_time"`
		SentTransactionCountLimit uint64 `mapstructure:"sent_transaction_count_limit"`
	} `mapstructure:"blockchains"`

	Task struct {
		StakeAmount       uint64 `mapstructure:"stake_amount" description:"stake amount, in ether unit"`
		DistanceThreshold uint64 `mapstructure:"distance_threshold"`
	}

	TaskSchema struct {
		StableDiffusionInference    string `mapstructure:"stable_diffusion_inference"`
		GPTInference                string `mapstructure:"gpt_inference"`
		StableDiffusionFinetuneLora string `mapstructure:"stable_diffusion_finetune_lora"`
	} `mapstructure:"task_schema"`

	Withdraw struct {
		RelayWalletAddress   string `mapstructure:"relay_wallet_address"`
		MinWithdrawalAmount  uint64 `mapstructure:"min_withdrawal_amount"`
		WithdrawalFee        uint64 `mapstructure:"withdrawal_fee"`
		WithdrawalFeeAddress string `mapstructure:"withdrawal_fee_address"`
	} `mapstructure:"withdraw"`

	Credits struct {
		APIAuthAddress string `mapstructure:"api_auth_address"`
	} `mapstructure:"credits"`

	Dao struct {
		TaskFeeShareAddress string `mapstructure:"task_fee_share_address"`
		TaskFeeSharePercent uint64 `mapstructure:"task_fee_share_percent"`
	} `mapstructure:"dao"`

	RelayAccount struct {
		DepositAddress string `mapstructure:"deposit_address"`
	} `mapstructure:"relay_account"`

	MAC struct {
		SecretKey     string `mapstructure:"secret_key"`
		SecretKeyFile string `mapstructure:"secret_key_file"`
	} `mapstructure:"mac"`

	QoS struct {
		ScorePoolSize               uint64  `mapstructure:"score_pool_size"`
		KickoutThreshold            float64 `mapstructure:"kickout_threshold"`
		RejoinQosLongFloor          float64 `mapstructure:"rejoin_qos_long_floor"`
		PenaltyFactor               float64 `mapstructure:"penalty_factor"`
		FirstTimeoutPenaltyFactor   float64 `mapstructure:"first_timeout_penalty_factor"`
		FirstTimeoutHealthThreshold float64 `mapstructure:"first_timeout_health_threshold"`
		SuccessBoost                float64 `mapstructure:"success_boost"`
		RecoveryTauMinutes          float64 `mapstructure:"recovery_tau_minutes"`
		HealthKickoutThreshold      float64 `mapstructure:"health_kickout_threshold"`
	} `mapstructure:"qos"`
}
