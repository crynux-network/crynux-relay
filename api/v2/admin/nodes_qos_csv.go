package admin

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type exportNodeQosCSVRow struct {
	Address      string
	Card         string
	ProbWeight   float64
	StakingScore float64
	QOSScore     float64
	QOSLong      float64
	QOSShort     float64
}

func ExportNodeQosCSV(c *gin.Context) {
	var nodes []models.Node
	if err := config.GetDB().Model(&models.Node{}).
		Where("status != ? AND status != ?", models.NodeStatusQuit, models.NodeStatusPaused).
		Find(&nodes).Error; err != nil {
		c.JSON(500, gin.H{
			"message": err.Error(),
		})
		return
	}

	rows := make([]exportNodeQosCSVRow, 0, len(nodes))
	now := time.Now().UTC()
	for _, node := range nodes {
		qosLong, qosShort, qosScore := service.CalculateQosComponents(node.QOSScore, node.HealthBase, node.HealthUpdatedAt)
		stakingScore, _, probWeight := service.CalculateSelectingProb(service.GetNodeScoreStakeAmount(node, now), service.GetMaxStaking(), qosScore)
		rows = append(rows, exportNodeQosCSVRow{
			Address:      node.Address,
			Card:         fmt.Sprintf("%s + %dGB", node.GPUName, node.GPUVram),
			ProbWeight:   probWeight,
			StakingScore: stakingScore,
			QOSScore:     qosScore,
			QOSLong:      qosLong,
			QOSShort:     qosShort,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ProbWeight > rows[j].ProbWeight
	})

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=active_nodes_qos.csv")

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{
		"node address",
		"card",
		"prob_weight",
		"staking_score",
		"qos_score",
		"qos_long",
		"qos_short",
	}); err != nil {
		c.JSON(500, gin.H{
			"message": err.Error(),
		})
		return
	}

	for _, row := range rows {
		if err := writer.Write([]string{
			row.Address,
			row.Card,
			strconv.FormatFloat(row.ProbWeight, 'f', 8, 64),
			strconv.FormatFloat(row.StakingScore, 'f', 8, 64),
			strconv.FormatFloat(row.QOSScore, 'f', 8, 64),
			strconv.FormatFloat(row.QOSLong, 'f', 8, 64),
			strconv.FormatFloat(row.QOSShort, 'f', 8, 64),
		}); err != nil {
			c.JSON(500, gin.H{
				"message": err.Error(),
			})
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(500, gin.H{
			"message": err.Error(),
		})
		return
	}
}
