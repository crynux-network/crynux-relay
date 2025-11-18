package service

import (
	"context"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func addNodeIncentive(ctx context.Context, db *gorm.DB, nodeAddress string, incentive float64, taskType models.TaskType) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	t := time.Now().UTC().Truncate(24 * time.Hour)
	nodeIncentive := models.NodeIncentive{Time: t, NodeAddress: nodeAddress}
	if err := db.WithContext(ctx).Model(&nodeIncentive).Where(&nodeIncentive).First(&nodeIncentive).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
	}
	if nodeIncentive.ID > 0 {
		nodeIncentive.Incentive += incentive
		nodeIncentive.TaskCount += 1
		if taskType == models.TaskTypeSD {
			nodeIncentive.SDTaskCount += 1
		} else if taskType == models.TaskTypeLLM {
			nodeIncentive.LLMTaskCount += 1
		} else if taskType == models.TaskTypeSDFTLora {
			nodeIncentive.SDFTLoraTaskCount += 1
		}
		if err := db.WithContext(dbCtx).Save(&nodeIncentive).Error; err != nil {
			return err
		}
	} else {
		nodeIncentive.Incentive = incentive
		nodeIncentive.TaskCount = 1
		if taskType == models.TaskTypeSD {
			nodeIncentive.SDTaskCount = 1
		} else if taskType == models.TaskTypeLLM {
			nodeIncentive.LLMTaskCount = 1
		} else if taskType == models.TaskTypeSDFTLora {
			nodeIncentive.SDFTLoraTaskCount = 1
		}
		if err := db.WithContext(dbCtx).Create(&nodeIncentive).Error; err != nil {
			return err
		}
	}
	return nil
}

func addNodeEarning(ctx context.Context, db *gorm.DB, nodeAddress string, operatorEarning, delegatorEarning *big.Int) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	t := time.Now().UTC().Truncate(24 * time.Hour)
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var nodeEarning models.NodeEarning
		// find today's node earning
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.NodeEarning{}).Where("node_address = ?", nodeAddress).Where("time = ?", t).First(&nodeEarning).Error; err != nil {
			// if today's node earning doesn't exist, create a new node earning
			if errors.Is(err, gorm.ErrRecordNotFound) {
				nodeEarning = models.NodeEarning{
					NodeAddress:      nodeAddress,
					Time:             sql.NullTime{Time: t, Valid: true},
					OperatorEarning:  models.BigInt{Int: *big.NewInt(0)},
					DelegatorEarning: models.BigInt{Int: *big.NewInt(0)},
				}
			} else {
				return err
			}
		}
		nodeEarning.OperatorEarning = models.BigInt{Int: *big.NewInt(0).Add(&nodeEarning.OperatorEarning.Int, operatorEarning)}
		nodeEarning.DelegatorEarning = models.BigInt{Int: *big.NewInt(0).Add(&nodeEarning.DelegatorEarning.Int, delegatorEarning)}
		if err := tx.Save(&nodeEarning).Error; err != nil {
			return err
		}

		// update the total node earning
		var totalNodeEarning models.NodeEarning
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.NodeEarning{}).Where("node_address = ?", nodeAddress).Where("time IS NULL").First(&totalNodeEarning).Error; err != nil {
			// if total node earning doesn't exist, create a new total node earning
			if errors.Is(err, gorm.ErrRecordNotFound) {
				totalNodeEarning = models.NodeEarning{
					NodeAddress:      nodeAddress,
					Time:             sql.NullTime{Valid: false},
					OperatorEarning:  models.BigInt{Int: *big.NewInt(0)},
					DelegatorEarning: models.BigInt{Int: *big.NewInt(0)},
				}
			} else {
				return err
			}
		}
		totalNodeEarning.OperatorEarning = models.BigInt{Int: *big.NewInt(0).Add(&totalNodeEarning.OperatorEarning.Int, operatorEarning)}
		totalNodeEarning.DelegatorEarning = models.BigInt{Int: *big.NewInt(0).Add(&totalNodeEarning.DelegatorEarning.Int, delegatorEarning)}
		if err := tx.Save(&totalNodeEarning).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func addUserStakingEarning(ctx context.Context, db *gorm.DB, userAddress, nodeAddress, network string, earning *big.Int) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	t := time.Now().UTC().Truncate(24 * time.Hour)
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userStakingEarning models.UserStakingEarning
		// find today's user staking earning
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.UserStakingEarning{}).Where("user_address = ?", userAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).Where("time = ?", t).First(&userStakingEarning).Error; err != nil {
			// if today's user staking earning doens't exist, create a new user staking earning
			if errors.Is(err, gorm.ErrRecordNotFound) {
				userStakingEarning = models.UserStakingEarning{
					UserAddress: userAddress,
					NodeAddress: nodeAddress,
					Time:        sql.NullTime{Time: t, Valid: true},
					Earning:     models.BigInt{Int: *big.NewInt(0)},
					Network:     network,
				}
			} else {
				return err
			}
		}
		userStakingEarning.Earning = models.BigInt{Int: *big.NewInt(0).Add(&userStakingEarning.Earning.Int, earning)}
		if err := tx.Save(&userStakingEarning).Error; err != nil {
			return err
		}

		// update the total user staking earning
		var totalUserStakingEarning models.UserStakingEarning
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.UserStakingEarning{}).Where("user_address = ?", userAddress).Where("node_address = ?", nodeAddress).Where("network = ?", network).Where("time IS NULL").First(&totalUserStakingEarning).Error; err != nil {
			// if total user staking earning doesn't exist, create a new total user staking earning
			if errors.Is(err, gorm.ErrRecordNotFound) {
				totalUserStakingEarning = models.UserStakingEarning{
					UserAddress: userAddress,
					NodeAddress: nodeAddress,
					Time:        sql.NullTime{Valid: false},
					Earning:     models.BigInt{Int: *big.NewInt(0)},
					Network:     network,
				}
			} else {
				return err
			}
		}
		totalUserStakingEarning.Earning = models.BigInt{Int: *big.NewInt(0).Add(&totalUserStakingEarning.Earning.Int, earning)}
		if err := tx.Save(&totalUserStakingEarning).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func addUserEarning(ctx context.Context, db *gorm.DB, userAddress string, earning *big.Int) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	t := time.Now().UTC().Truncate(24 * time.Hour)
	if err := db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		var userEarning models.UserEarning
		// find today's user earning
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.UserEarning{}).Where("user_address = ?", userAddress).Where("time = ?", t).First(&userEarning).Error; err != nil {
			// if today's user earning doens't exist, create a new user earning
			if errors.Is(err, gorm.ErrRecordNotFound) {
				userEarning = models.UserEarning{
					UserAddress: userAddress,
					Time:        sql.NullTime{Time: t, Valid: true},
					Earning:     models.BigInt{Int: *big.NewInt(0)},
				}
			} else {
				return err
			}
		}
		userEarning.Earning = models.BigInt{Int: *big.NewInt(0).Add(&userEarning.Earning.Int, earning)}
		if err := tx.Save(&userEarning).Error; err != nil {
			return err
		}

		// update the total user earning
		var totalUserEarning models.UserEarning
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&models.UserEarning{}).Where("user_address = ?", userAddress).Where("time IS NULL").First(&totalUserEarning).Error; err != nil {
			// if total user earning doesn't exist, create a new total user earning
			if errors.Is(err, gorm.ErrRecordNotFound) {
				totalUserEarning = models.UserEarning{
					UserAddress: userAddress,
					Time:        sql.NullTime{Valid: false},
					Earning:     models.BigInt{Int: *big.NewInt(0)},
				}
			} else {
				return err
			}
		}
		totalUserEarning.Earning = models.BigInt{Int: *big.NewInt(0).Add(&totalUserEarning.Earning.Int, earning)}
		if err := tx.Save(&totalUserEarning).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
