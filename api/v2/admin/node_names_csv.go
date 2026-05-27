package admin

import (
	"crynux_relay/config"
	"crynux_relay/service"
	"encoding/csv"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ExportNodeNamesCSV(c *gin.Context) {
	entries, err := service.ListNodeNameCountsFromCache(c.Request.Context(), config.GetDB())
	if err != nil {
		c.JSON(500, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=node_names.csv")

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{"gpu_name", "gpu_vram", "node_version", "active_count"}); err != nil {
		c.JSON(500, gin.H{
			"message": err.Error(),
		})
		return
	}
	for _, row := range buildNodeNamesCSVRows(entries) {
		if err := writer.Write(row); err != nil {
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

func buildNodeNamesCSVRows(entries []service.NodeNameCountEntry) [][]string {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{
			entry.GPUName,
			strconv.FormatUint(entry.GPUVram, 10),
			entry.NodeVersion,
			strconv.FormatUint(entry.ActiveCount, 10),
		})
	}
	return rows
}
