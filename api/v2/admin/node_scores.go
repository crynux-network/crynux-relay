package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/service"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/params"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GetNodeScoresInput struct {
	Address string `path:"address" validate:"required"`
}

type NodeScoresData struct {
	Address          string  `json:"address"`
	Network          string  `json:"network"`
	OperatorStaking  string  `json:"operator_staking"`
	DelegatorStaking string  `json:"delegator_staking"`
	VestingStaking   string  `json:"vesting_staking"`
	ScoreStaking     string  `json:"score_staking"`
	MaxStaking       string  `json:"max_staking"`
	VestingScore     float64 `json:"vesting_score"`
	StakingScore     float64 `json:"staking_score"`
	QOSScore         float64 `json:"qos_score"`
	ProbWeight       float64 `json:"prob_weight"`
}

type GetNodeScoresResponse struct {
	response.Response
	Data NodeScoresData `json:"data"`
}

func GetNodeScores(c *gin.Context, in *GetNodeScoresInput) (*GetNodeScoresResponse, error) {
	nodeAddress, node, err := getNodeByAdminAddress(c, in.Address)
	if err != nil {
		if errors.Is(err, errInvalidAdminNodeAddress) {
			return nil, &response.ErrorResponse{Response: response.Response{Message: "invalid node address"}}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}

	now := time.Now().UTC()
	operatorStaking := &node.StakeAmount.Int
	delegatorStaking := service.GetNodeTotalStakeAmount(node.Address, node.Network)
	vestingStaking := service.GetNodeLockedVestingAmount(node.Address, now)
	scoreStaking := service.GetNodeScoreStakeAmount(*node, now)
	maxStaking := service.GetMaxStaking()
	qosScore := service.CalculateQosScore(node.QOSScore, node.HealthBase, node.HealthUpdatedAt)
	stakingScore, _, probWeight := service.CalculateSelectingProb(scoreStaking, maxStaking, qosScore)
	vestingScore := service.CalculateStakingScore(vestingStaking, maxStaking)

	return &GetNodeScoresResponse{
		Data: NodeScoresData{
			Address:          nodeAddress,
			Network:          node.Network,
			OperatorStaking:  formatCNXAmount4(operatorStaking),
			DelegatorStaking: formatCNXAmount4(delegatorStaking),
			VestingStaking:   formatCNXAmount4(vestingStaking),
			ScoreStaking:     formatCNXAmount4(scoreStaking),
			MaxStaking:       formatCNXAmount4(maxStaking),
			VestingScore:     vestingScore,
			StakingScore:     stakingScore,
			QOSScore:         qosScore,
			ProbWeight:       probWeight,
		},
	}, nil
}

func formatCNXAmount4(amount *big.Int) string {
	if amount == nil {
		amount = big.NewInt(0)
	}

	weiPerCNX := big.NewInt(params.Ether)
	integer := new(big.Int).Quo(amount, weiPerCNX)
	remainder := new(big.Int).Mod(amount, weiPerCNX)
	fraction := new(big.Int).Quo(remainder, big.NewInt(100000000000000))
	return fmt.Sprintf("%s.%04d", integer.String(), fraction.Int64())
}
