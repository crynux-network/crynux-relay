package network

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/config"
	"sort"

	"github.com/gin-gonic/gin"
)

type WithdrawalFeeTier struct {
	MinAmount uint64  `json:"min_amount" description:"tier lower bound of withdraw amount, in ether unit"`
	FeeRatio  float64 `json:"fee_ratio" description:"proportional fee ratio applied to the whole withdraw amount in this tier"`
}

type NetworkWithdrawConfig struct {
	Network            string              `json:"network"`
	TokenType          string              `json:"token_type"`
	WithdrawalFee      uint64              `json:"withdrawal_fee" description:"fixed withdrawal fee, in ether unit"`
	WithdrawalMin      uint64              `json:"withdrawal_min" description:"minimum withdraw amount, in ether unit"`
	WithdrawalFeeTiers []WithdrawalFeeTier `json:"withdrawal_fee_tiers"`
}

type GetWithdrawConfigResponse struct {
	response.Response
	Data []NetworkWithdrawConfig `json:"data"`
}

func GetWithdrawConfig(_ *gin.Context) (*GetWithdrawConfigResponse, error) {
	appConfig := config.GetConfig()
	networks, err := appConfig.EffectiveFundingNetworks()
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	data := make([]NetworkWithdrawConfig, 0, len(networks))
	for _, networkConfig := range networks {
		tiers := make([]WithdrawalFeeTier, 0, len(networkConfig.WithdrawalFeeTiers))
		for _, tier := range networkConfig.WithdrawalFeeTiers {
			tiers = append(tiers, WithdrawalFeeTier{
				MinAmount: tier.MinAmount,
				FeeRatio:  tier.FeeRatio,
			})
		}
		data = append(data, NetworkWithdrawConfig{
			Network:            networkConfig.Network,
			TokenType:          networkConfig.TokenType,
			WithdrawalFee:      networkConfig.WithdrawalFee,
			WithdrawalMin:      networkConfig.WithdrawalMin,
			WithdrawalFeeTiers: tiers,
		})
	}
	sort.Slice(data, func(i, j int) bool {
		return data[i].Network < data[j].Network
	})

	return &GetWithdrawConfigResponse{
		Data: data,
	}, nil
}
