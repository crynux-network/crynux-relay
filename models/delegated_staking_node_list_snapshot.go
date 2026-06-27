package models

import "time"

const (
	DelegatedStakingNodeStatusGroupRunning = "running"
	DelegatedStakingNodeStatusGroupStopped = "stopped"
)

type DelegatedStakingNodeListSnapshot struct {
	ID                 uint `gorm:"primarykey"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	NodeAddress        string     `gorm:"type:string;size:191;not null;uniqueIndex;index"`
	Network            string     `gorm:"type:string;size:64;not null;index"`
	Status             NodeStatus `gorm:"not null;index"`
	StatusGroup        string     `gorm:"type:string;size:32;not null;index"`
	StatusRank         uint8      `gorm:"not null;index"`
	GPUName            string     `gorm:"type:string;size:191;not null;index"`
	GPUVram            uint64     `gorm:"not null;index"`
	Version            string     `gorm:"type:string;size:64;not null;index"`
	OperatorEmission4w BigInt     `gorm:"column:operator_emission_4w;type:decimal(65,0);not null;default:0;index"`
	OperatorStaking    BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegatorStaking   BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	TotalStaking       BigInt     `gorm:"type:decimal(65,0);not null;default:0;index"`
	DelegatorsNum      uint64     `gorm:"not null;default:0;index"`
	ProbWeight         float64    `gorm:"not null;default:0;index"`
	QOS                float64    `gorm:"column:qos;not null;default:0;index"`
}
