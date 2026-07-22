package incentive

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type GetDelegationIncentiveParams struct{}

type DelegationIncentive struct {
	DelegatorAddress string  `json:"delegator_address"`
	NodeAddress      string  `json:"node_address"`
	Network          string  `json:"network"`
	StakingAmount    string  `json:"staking_amount"`
	TaskFee          string  `json:"task_fee"`
	DelegationApr12m float64 `json:"delegation_apr_12m"`
}

type GetDelegationIncentiveData struct {
	Delegations []DelegationIncentive `json:"delegations"`
}

type GetDelegationIncentiveOutput struct {
	Data *GetDelegationIncentiveData `json:"data"`
}

func GetDelegationIncentive(c *gin.Context, input *GetDelegationIncentiveParams) (*GetDelegationIncentiveOutput, error) {
	snapshots, err := service.GetDelegationTaskFeeLeaderboard(c.Request.Context(), config.GetDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	delegations := make([]DelegationIncentive, 0, len(snapshots))
	for _, snapshot := range snapshots {
		delegations = append(delegations, DelegationIncentive{
			DelegatorAddress: snapshot.DelegatorAddress,
			NodeAddress:      snapshot.NodeAddress,
			Network:          snapshot.Network,
			StakingAmount:    snapshot.StakingAmount.String(),
			TaskFee:          snapshot.TaskFee.String(),
			DelegationApr12m: snapshot.DelegationApr12m,
		})
	}

	return &GetDelegationIncentiveOutput{
		Data: &GetDelegationIncentiveData{
			Delegations: delegations,
		},
	}, nil
}
