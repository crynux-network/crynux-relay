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
	Address     string
	Type        string
	TaskFee     *big.Int
	NodeAddress string
	Network     string
}

type emissionTaskFeeAggregateRow struct {
	Address string
	TaskFee string
}

type emissionTaskFeeDelegationDetailRow struct {
	UserAddress string
	NodeAddress string
	Network     string
	TaskFee     string
}

type emissionTaskFeeCSVRow struct {
	Address   string
	Type      string
	TaskFee   string
	Emission  string
	StartTime string

	NodeAddress string
	Network     string
}

func ExportEmissionTaskFeeCSV(c *gin.Context) {
	weekInfo, err := getPreviousEmissionWeekInfo()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	emissionStartTime := fmt.Sprintf("%d", weekInfo.WeekEndDate.Unix())
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
				if participants[i].Address == participants[j].Address {
					if participants[i].NodeAddress == participants[j].NodeAddress {
						return participants[i].Network < participants[j].Network
					}
					return participants[i].NodeAddress < participants[j].NodeAddress
				}
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

	rows := buildEmissionTaskFeeCSVRows(participants, totalTaskFee, weekInfo.NodeEmissionPoolCNX, emissionStartTime)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=emission_task_fee_%s_%s.csv", weekInfo.WeekStartDate.Format("20060102"), weekInfo.WeekEndDate.AddDate(0, 0, -1).Format("20060102")))

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{"address", "type", "task fee", "emission", "start_time", "node_address", "network"}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	for _, row := range rows {
		if err := writer.Write([]string{row.Address, row.Type, row.TaskFee, row.Emission, row.StartTime, row.NodeAddress, row.Network}); err != nil {
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

	delegatorRows := make([]emissionTaskFeeDelegationDetailRow, 0)
	if err := db.Model(&models.UserStakingEarning{}).
		Select("user_address, node_address, network, SUM(CAST(earning AS DECIMAL(65,0))) as task_fee").
		Where("time >= ? AND time < ?", weekStart, weekEnd).
		Group("user_address, node_address, network").
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
			Address:     row.UserAddress,
			Type:        models.VestingTypeDelegation,
			TaskFee:     taskFee,
			NodeAddress: row.NodeAddress,
			Network:     row.Network,
		})
	}

	return participants, nil
}

func buildEmissionTaskFeeCSVRows(participants []emissionTaskFeeParticipant, totalTaskFee *big.Int, nodeEmissionPoolCNX int64, emissionStartTime string) []emissionTaskFeeCSVRow {
	rows := make([]emissionTaskFeeCSVRow, 0, len(participants)+1)
	remainingEmission := big.NewInt(0).Mul(big.NewInt(nodeEmissionPoolCNX), big.NewInt(1_000_000))

	if totalTaskFee.Sign() > 0 {
		pool := big.NewInt(0).Set(remainingEmission)
		for _, p := range participants {
			numerator := big.NewInt(0).Mul(p.TaskFee, pool)
			emissionMicroCNX := big.NewInt(0).Div(numerator, totalTaskFee)
			remainingEmission.Sub(remainingEmission, emissionMicroCNX)
			rows = append(rows, emissionTaskFeeCSVRow{
				Address:     p.Address,
				Type:        p.Type,
				TaskFee:     formatEmissionDecimalCNX(p.TaskFee, big.NewInt(1_000_000_000_000)),
				Emission:    formatMicroCNX(emissionMicroCNX),
				StartTime:   emissionStartTime,
				NodeAddress: p.NodeAddress,
				Network:     p.Network,
			})
		}
	}

	rows = append(rows, emissionTaskFeeCSVRow{
		Address:   "",
		Type:      "remainder",
		TaskFee:   "0.000000",
		Emission:  formatMicroCNX(remainingEmission),
		StartTime: emissionStartTime,
	})

	return rows
}

func getPreviousEmissionWeekInfo() (*service.EmissionWeekInfo, error) {
	return service.GetPreviousEmissionWeekInfo(time.Now().UTC(), config.GetConfig().Dao.MainnetStartTime)
}

func formatEmissionDecimalCNX(amount *big.Int, unit *big.Int) string {
	scaled := big.NewInt(0).Div(amount, unit)
	return formatMicroCNX(scaled)
}

func formatMicroCNX(amount *big.Int) string {
	whole := big.NewInt(0).Div(amount, big.NewInt(1_000_000))
	fraction := big.NewInt(0).Mod(amount, big.NewInt(1_000_000))
	return fmt.Sprintf("%s.%06d", whole.String(), fraction.Int64())
}
