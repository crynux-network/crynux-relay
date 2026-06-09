package admin

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"encoding/csv"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type emissionTaskFeeParticipant struct {
	Address string
	Type    string
	TaskFee *big.Int
}

type emissionTaskFeeAggregateRow struct {
	Address string
	TaskFee string
}

type emissionTaskFeeCSVRow struct {
	Address  string
	Type     string
	TaskFee  string
	Emission string
}

func ExportEmissionTaskFeeCSV(c *gin.Context) {
	weekInfo, err := getPreviousEmissionWeekInfo()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	participants, err := loadEmissionTaskFeeParticipants(weekInfo.WeekStartDate, weekInfo.WeekEndDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	sort.Slice(participants, func(i, j int) bool {
		cmp := participants[i].TaskFee.Cmp(participants[j].TaskFee)
		if cmp == 0 {
			if participants[i].Type == participants[j].Type {
				return participants[i].Address < participants[j].Address
			}
			return participants[i].Type < participants[j].Type
		}
		return cmp > 0
	})

	totalTaskFee := big.NewInt(0)
	for _, p := range participants {
		totalTaskFee.Add(totalTaskFee, p.TaskFee)
	}

	rows := buildEmissionTaskFeeCSVRows(participants, totalTaskFee, weekInfo.NodeEmissionPoolCNX)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=emission_task_fee_%s_%s.csv", weekInfo.WeekStartDate.Format("20060102"), weekInfo.WeekEndDate.AddDate(0, 0, -1).Format("20060102")))

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{"address", "type", "task fee", "emission"}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	for _, row := range rows {
		if err := writer.Write([]string{row.Address, row.Type, row.TaskFee, row.Emission}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}
}

func loadEmissionTaskFeeParticipants(weekStart, weekEnd time.Time) ([]emissionTaskFeeParticipant, error) {
	db := config.GetDB()

	nodeRows := make([]emissionTaskFeeAggregateRow, 0)
	if err := db.Model(&models.NodeEarning{}).
		Select("node_address as address, SUM(CAST(operator_earning AS DECIMAL(65,0))) as task_fee").
		Where("time >= ? AND time < ?", weekStart, weekEnd).
		Group("node_address").
		Scan(&nodeRows).Error; err != nil {
		return nil, err
	}

	delegatorRows := make([]emissionTaskFeeAggregateRow, 0)
	if err := db.Model(&models.UserEarning{}).
		Select("user_address as address, SUM(CAST(earning AS DECIMAL(65,0))) as task_fee").
		Where("time >= ? AND time < ?", weekStart, weekEnd).
		Group("user_address").
		Scan(&delegatorRows).Error; err != nil {
		return nil, err
	}

	participants := make([]emissionTaskFeeParticipant, 0, len(nodeRows)+len(delegatorRows))
	for _, row := range nodeRows {
		taskFee, ok := big.NewInt(0).SetString(row.TaskFee, 10)
		if !ok || taskFee.Sign() <= 0 {
			continue
		}
		participants = append(participants, emissionTaskFeeParticipant{
			Address: row.Address,
			Type:    "node",
			TaskFee: taskFee,
		})
	}

	for _, row := range delegatorRows {
		taskFee, ok := big.NewInt(0).SetString(row.TaskFee, 10)
		if !ok || taskFee.Sign() <= 0 {
			continue
		}
		participants = append(participants, emissionTaskFeeParticipant{
			Address: row.Address,
			Type:    models.VestingTypeDelegation,
			TaskFee: taskFee,
		})
	}

	return participants, nil
}

func buildEmissionTaskFeeCSVRows(participants []emissionTaskFeeParticipant, totalTaskFee *big.Int, nodeEmissionPoolCNX int64) []emissionTaskFeeCSVRow {
	rows := make([]emissionTaskFeeCSVRow, 0, len(participants)+1)
	remainingEmission := nodeEmissionPoolCNX

	if totalTaskFee.Sign() > 0 {
		pool := big.NewInt(nodeEmissionPoolCNX)
		for _, p := range participants {
			numerator := big.NewInt(0).Mul(p.TaskFee, pool)
			emissionCNX := big.NewInt(0).Div(numerator, totalTaskFee).Int64()
			remainingEmission -= emissionCNX
			rows = append(rows, emissionTaskFeeCSVRow{
				Address:  p.Address,
				Type:     p.Type,
				TaskFee:  formatCNXAmount(p.TaskFee),
				Emission: formatIntegerCNX(emissionCNX),
			})
		}
	}

	rows = append(rows, emissionTaskFeeCSVRow{
		Address:  "",
		Type:     "remainder",
		TaskFee:  "0.00",
		Emission: formatIntegerCNX(remainingEmission),
	})

	return rows
}

func getPreviousEmissionWeekInfo() (*service.EmissionWeekInfo, error) {
	return service.GetPreviousEmissionWeekInfo(time.Now().UTC(), config.GetConfig().Dao.MainnetStartTime)
}

func formatIntegerCNX(amount int64) string {
	return fmt.Sprintf("%d.00", amount)
}
