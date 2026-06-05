package config

import "fmt"

func (cfg *AppConfig) GetEffectiveFundingNetwork(network string) (EffectiveFundingNetworkConfig, bool) {
	if blockchain, ok := cfg.Blockchains[network]; ok {
		return EffectiveFundingNetworkConfig{
			Network:        network,
			TokenType:      FundingTokenTypeNative,
			RPS:            blockchain.RPS,
			RpcEndpoint:    blockchain.RpcEndpoint,
			StartBlockNum:  blockchain.StartBlockNum,
			ChainID:        blockchain.ChainID,
			BenefitAddress: blockchain.Contracts.BenefitAddress,
			WithdrawalFee:  blockchain.WithdrawalFee,
			WithdrawalMin:  blockchain.WithdrawalMin,
		}, true
	}

	if networkConfig, ok := cfg.DepositWithdrawNetworks[network]; ok {
		return EffectiveFundingNetworkConfig{
			Network:        network,
			TokenType:      FundingTokenTypeERC20,
			RPS:            networkConfig.RPS,
			RpcEndpoint:    networkConfig.RpcEndpoint,
			StartBlockNum:  networkConfig.StartBlockNum,
			ChainID:        networkConfig.ChainID,
			BenefitAddress: networkConfig.Contracts.BenefitAddress,
			TokenAddress:   networkConfig.Contracts.TokenAddress,
			LogBlockRange:  networkConfig.LogBlockRange,
			WithdrawalFee:  networkConfig.WithdrawalFee,
			WithdrawalMin:  networkConfig.WithdrawalMin,
		}, true
	}

	return EffectiveFundingNetworkConfig{}, false
}

func (cfg *AppConfig) AllBlockchainNetworks() (map[string]EffectiveFundingNetworkConfig, error) {
	networks := make(map[string]EffectiveFundingNetworkConfig, len(cfg.Blockchains)+len(cfg.DepositWithdrawNetworks))
	for network := range cfg.Blockchains {
		networkConfig, _ := cfg.GetEffectiveFundingNetwork(network)
		networks[network] = networkConfig
	}
	for network := range cfg.DepositWithdrawNetworks {
		if _, exists := networks[network]; exists {
			return nil, fmt.Errorf("network %s cannot be configured in both blockchains and deposit_withdraw_networks", network)
		}
		networkConfig, _ := cfg.GetEffectiveFundingNetwork(network)
		networks[network] = networkConfig
	}
	return networks, nil
}

func (cfg *AppConfig) EffectiveFundingNetworks() (map[string]EffectiveFundingNetworkConfig, error) {
	return cfg.AllBlockchainNetworks()
}
