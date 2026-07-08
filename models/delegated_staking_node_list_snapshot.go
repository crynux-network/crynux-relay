package models

import "time"

const (
	DelegatedStakingNodeStatusGroupRunning = "running"
	DelegatedStakingNodeStatusGroupStopped = "stopped"
)

type DelegatedStakingNodeListSnapshot struct {
	ID                                 uint `gorm:"primarykey"`
	CreatedAt                          time.Time
	UpdatedAt                          time.Time
	NodeAddress                        string     `gorm:"type:string;size:191;not null;uniqueIndex:idx_delegated_staking_node_snapshots_address"`
	Network                            string     `gorm:"type:string;size:64;not null;index"`
	Status                             NodeStatus `gorm:"not null;index"`
	StatusGroup                        string     `gorm:"type:string;size:32;not null;index"`
	StatusRank                         uint8      `gorm:"not null;index"`
	GPUName                            string     `gorm:"type:string;size:191;not null;index"`
	GPUVram                            uint64     `gorm:"not null;index"`
	Version                            string     `gorm:"type:string;size:64;not null;index"`
	OperatorEmission4w                 BigInt     `gorm:"column:operator_emission_4w;type:decimal(65,0);not null;default:0;index"`
	DelegatorEmission4w                BigInt     `gorm:"column:delegator_emission_4w;type:decimal(65,0);not null;default:0;index"`
	OperatorStaking                    BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegatorStaking                   BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	TotalStaking                       BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegatorsNum                      uint64     `gorm:"not null;default:0;index"`
	ProbWeight                         float64    `gorm:"not null;default:0;index"`
	QOS                                float64    `gorm:"column:qos;not null;default:0;index"`
	EstimatedUpcomingOperatorEmission  BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	EstimatedUpcomingDelegatorEmission BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegationApr12m                   float64    `gorm:"column:delegation_apr_12m;not null;default:0;index"`
	EstimatedNext10kDelegationApr      float64    `gorm:"column:estimated_next_10k_delegation_apr;not null;default:0;index"`
	EstimatedNext100kDelegationApr     float64    `gorm:"column:estimated_next_100k_delegation_apr;not null;default:0;index"`
	EstimatedNext1mDelegationApr       float64    `gorm:"column:estimated_next_1m_delegation_apr;not null;default:0;index"`
	AprObservationDays                 uint32     `gorm:"not null;default:0"`
	DelegationAprUpdatedAt             time.Time  `gorm:"not null"`
}
