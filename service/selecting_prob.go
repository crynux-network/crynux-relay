package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/utils"
	"math/big"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	globalMaxStaking  = newMaxStaking()
	globalMaxQosScore = float64(TASK_SCORE_REWARDS[0])
)

func InitSelectingProb(ctx context.Context, db *gorm.DB) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var nodes []models.Node
	if err := db.WithContext(dbCtx).Model(&models.Node{}).Where("status != ?", models.NodeStatusQuit).Find(&nodes).Error; err != nil {
		return err
	}

	stakingMap := make(map[string]*big.Int)
	for _, node := range nodes {
		addr := node.Address
		amount := big.NewInt(0).Add(&node.StakeAmount.Int, GetNodeTotalStakeAmount(addr, node.Network))
		stakingMap[addr] = amount
	}

	globalMaxStaking.init(stakingMap)
	return nil
}

func CalculateSelectingProb(staking, maxStaking *big.Int, qosScore, maxQosScore float64) (float64, float64, float64) {
	stakingProb := CalculateStakingScore(staking, maxStaking)
	qosProb := CalculateQosScore(qosScore, maxQosScore)
	if qosProb == 0 {
		qosProb = 0.5
	}
	var prob float64
	if stakingProb == 0 || qosProb == 0 {
		prob = 0
	} else {
		prob = stakingProb * qosProb / (stakingProb + qosProb)
	}
	return stakingProb, qosProb, prob
}

func CalculateStakingScore(staking, maxStaking *big.Int) float64 {
	if maxStaking.Sign() == 0 {
		return 0
	}
	p := big.NewFloat(0).Quo(big.NewFloat(0).SetInt(staking), big.NewFloat(0).SetInt(maxStaking))
	p = big.NewFloat(0).Sqrt(p)
	stakingProb, _ := p.Float64()
	return stakingProb
}

func CalculateQosScore(qosScore, maxQosScore float64) float64 {
	if maxQosScore == 0 {
		return 0
	}
	return qosScore / maxQosScore
}

func GetMaxStaking() *big.Int {
	return globalMaxStaking.get()
}

func GetMaxQosScore() float64 {
	return globalMaxQosScore
}

func UpdateMaxStaking(address string, staking *big.Int) {
	globalMaxStaking.update(address, staking)
}

type maxStaking struct {
	sync.RWMutex
	maxStaking *big.Int
	maxAddress string
	stakingMap map[string]*big.Int
}

func newMaxStaking() *maxStaking {
	return &maxStaking{
		maxStaking: big.NewInt(0),
		maxAddress: "",
		stakingMap: make(map[string]*big.Int),
	}
}

func (g *maxStaking) init(stakingMap map[string]*big.Int) {
	g.Lock()
	defer g.Unlock()

	for addr, amount := range stakingMap {
		amount = big.NewInt(0).Set(amount)
		g.stakingMap[addr] = amount
		if amount.Cmp(g.maxStaking) > 0 {
			g.maxAddress = addr
			g.maxStaking = amount
		}
	}
}

func (g *maxStaking) update(address string, staking *big.Int) {
	g.Lock()
	defer g.Unlock()

	copyStaking := big.NewInt(0).Set(staking)
	g.stakingMap[address] = copyStaking
	if staking.Cmp(g.maxStaking) > 0 {
		g.maxStaking = copyStaking
		g.maxAddress = address
	} else if address == g.maxAddress && staking.Cmp(g.maxStaking) < 0 {
		g.maxAddress = ""
		g.maxStaking = big.NewInt(0)
		for addr, amount := range g.stakingMap {
			if amount.Cmp(g.maxStaking) > 0 {
				g.maxAddress = addr
				g.maxStaking = amount
			}
		}
	}
}

func (g *maxStaking) get() *big.Int {
	g.RLock()
	defer g.RUnlock()
	amount := g.maxStaking
	if amount.Sign() == 0 {
		appConfig := config.GetConfig()
		amount = utils.EtherToWei(big.NewInt(int64(appConfig.Task.StakeAmount)))
	}
	return amount
}
