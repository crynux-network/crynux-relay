package admin

import (
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultTaskHistoryPageSize = 30
	maxTaskHistoryPageSize     = 200
)

type nodeTaskHistoryRow struct {
	TaskType              string
	Model                 string
	VRAMRequirement       string
	GroupValidated        string
	TaskStatus            string
	TaskExecutionDuration string
	ValidationDuration    string
	ResultUploadDuration  string
	CreatedAt             string
}

func ExportNodeTaskHistoryHTML(c *gin.Context) {
	nodeAddress := strings.TrimSpace(c.Query("address"))
	if nodeAddress == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "address query parameter is required",
		})
		return
	}

	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), defaultTaskHistoryPageSize)
	if pageSize > maxTaskHistoryPageSize {
		pageSize = maxTaskHistoryPageSize
	}

	db := config.GetDB().Model(&models.InferenceTask{}).Where("selected_node = ?", nodeAddress)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	pageCount := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pageCount == 0 {
		pageCount = 1
	}
	if page > pageCount {
		page = pageCount
	}

	var tasks []models.InferenceTask
	offset := (page - 1) * pageSize
	if err := db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	rows := make([]nodeTaskHistoryRow, 0, len(tasks))
	for _, task := range tasks {
		groupValidated := isGroupValidatedTask(task.Status)
		row := nodeTaskHistoryRow{
			TaskType:              taskTypeLabel(task.TaskType),
			Model:                 modelLabel(task.ModelIDs),
			VRAMRequirement:       formatVRAMRequirement(task.MinVRAM, task.RequiredGPUVRAM),
			GroupValidated:        yesNo(groupValidated),
			TaskStatus:            taskStatusLabel(task.Status, task.AbortReason),
			TaskExecutionDuration: durationSecondsLabel(task.StartTime, task.ScoreReadyTime),
			ValidationDuration:    validationDurationSecondsLabel(groupValidated, task.ScoreReadyTime, task.ValidatedTime),
			ResultUploadDuration:  durationSecondsLabel(task.ValidatedTime, task.ResultUploadedTime),
			CreatedAt:             task.CreatedAt.UTC().Format(time.RFC3339),
		}
		rows = append(rows, row)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, buildTaskHistoryHTML(c, nodeAddress, page, pageSize, pageCount, total, rows))
}

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func yesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func taskTypeLabel(taskType models.TaskType) string {
	switch taskType {
	case models.TaskTypeLLM:
		return "LLM"
	case models.TaskTypeSD:
		return "SD"
	case models.TaskTypeSDFTLora:
		return "SDFTLora"
	default:
		return fmt.Sprintf("Unknown(%d)", taskType)
	}
}

func taskStatusLabel(status models.TaskStatus, abortReason models.TaskAbortReason) string {
	switch status {
	case models.TaskEndSuccess, models.TaskEndGroupSuccess, models.TaskEndGroupRefund:
		return "Success"
	case models.TaskEndAborted:
		switch abortReason {
		case models.TaskAbortTimeout:
			return "Timeout"
		case models.TaskAbortModelDownloadFailed:
			return "Model Download Failed"
		case models.TaskAbortIncorrectResult:
			return "Incorrect Result"
		case models.TaskAbortTaskFeeTooLow:
			return "Task Fee Too Low"
		case models.TaskAbortGroupTimeout:
			return "Group Timeout"
		case models.TaskAbortErrorReported:
			return "Error Reported"
		default:
			return "Aborted"
		}
	case models.TaskEndInvalidated:
		return "Invalidated"
	case models.TaskErrorReported:
		return "Task Error Reported"
	case models.TaskScoreReady:
		return "Score Ready"
	case models.TaskValidated:
		return "Validated"
	case models.TaskGroupValidated:
		return "Group Validated"
	case models.TaskStarted:
		return "Started"
	case models.TaskParametersUploaded:
		return "Parameters Uploaded"
	case models.TaskQueued:
		return "Queued"
	default:
		return fmt.Sprintf("Status(%d)", status)
	}
}

func isGroupValidatedTask(status models.TaskStatus) bool {
	switch status {
	case models.TaskGroupValidated, models.TaskEndGroupSuccess, models.TaskEndGroupRefund:
		return true
	default:
		return false
	}
}

func modelLabel(modelIDs []string) string {
	filtered := make([]string, 0, len(modelIDs))
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return "-"
	}
	return strings.Join(filtered, ", ")
}

func formatVRAMRequirement(minVRAM, requiredGPUVRAM uint64) string {
	if requiredGPUVRAM > 0 {
		return fmt.Sprintf("%d MB", requiredGPUVRAM)
	}
	if minVRAM > 0 {
		return fmt.Sprintf("%d MB", minVRAM)
	}
	return "-"
}

func durationSecondsLabel(start, end sql.NullTime) string {
	if !start.Valid || !end.Valid {
		return "-"
	}
	seconds := int64(end.Time.Sub(start.Time).Seconds())
	if seconds < 0 {
		return "-"
	}
	return strconv.FormatInt(seconds, 10)
}

func validationDurationSecondsLabel(groupValidated bool, scoreReady, validated sql.NullTime) string {
	if !groupValidated {
		return "-"
	}
	return durationSecondsLabel(scoreReady, validated)
}

func buildTaskHistoryHTML(c *gin.Context, nodeAddress string, page, pageSize, pageCount int, total int64, rows []nodeTaskHistoryRow) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\">")
	b.WriteString("<title>Node Task History</title>")
	b.WriteString("<style>")
	b.WriteString("body{font-family:Segoe UI,Arial,sans-serif;margin:16px;color:#1f2328;}")
	b.WriteString("h1{margin:0 0 12px 0;font-size:22px;}")
	b.WriteString(".meta{margin-bottom:14px;color:#57606a;}")
	b.WriteString("table{border-collapse:collapse;width:100%;font-size:14px;}")
	b.WriteString("th,td{border:1px solid #d0d7de;padding:8px;vertical-align:top;}")
	b.WriteString("th{background:#f6f8fa;text-align:left;}")
	b.WriteString("tr:nth-child(even){background:#fbfdff;}")
	b.WriteString(".pager{margin-top:12px;display:flex;gap:8px;align-items:center;}")
	b.WriteString(".pager a{color:#0969da;text-decoration:none;}")
	b.WriteString(".pager a:hover{text-decoration:underline;}")
	b.WriteString("</style></head><body>")

	escapedAddress := html.EscapeString(nodeAddress)
	b.WriteString("<h1>Node Task History</h1>")
	b.WriteString(fmt.Sprintf("<div class=\"meta\">Node: <code>%s</code> | Total Tasks: %d | Page %d/%d | Page Size: %d</div>", escapedAddress, total, page, pageCount, pageSize))

	b.WriteString("<table><thead><tr>")
	b.WriteString("<th>Created At</th>")
	b.WriteString("<th>Task Type</th>")
	b.WriteString("<th>Model</th>")
	b.WriteString("<th>Vram Requirement</th>")
	b.WriteString("<th>Group Validated</th>")
	b.WriteString("<th>Task Status</th>")
	b.WriteString("<th>Task Execution Duration (s)</th>")
	b.WriteString("<th>Validation Duration (s)</th>")
	b.WriteString("<th>Result Upload Duration (s)</th>")
	b.WriteString("</tr></thead><tbody>")

	if len(rows) == 0 {
		b.WriteString("<tr><td colspan=\"9\">No task history found.</td></tr>")
	} else {
		for _, row := range rows {
			b.WriteString("<tr>")
			b.WriteString("<td>" + html.EscapeString(row.CreatedAt) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.TaskType) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.Model) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.VRAMRequirement) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.GroupValidated) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.TaskStatus) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.TaskExecutionDuration) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.ValidationDuration) + "</td>")
			b.WriteString("<td>" + html.EscapeString(row.ResultUploadDuration) + "</td>")
			b.WriteString("</tr>")
		}
	}
	b.WriteString("</tbody></table>")

	b.WriteString("<div class=\"pager\">")
	if page > 1 {
		b.WriteString("<a href=\"" + buildPageURL(c, nodeAddress, page-1, pageSize) + "\">Prev</a>")
	}
	if page < pageCount {
		b.WriteString("<a href=\"" + buildPageURL(c, nodeAddress, page+1, pageSize) + "\">Next</a>")
	}
	b.WriteString("</div>")

	b.WriteString("</body></html>")
	return b.String()
}

func buildPageURL(c *gin.Context, address string, page, pageSize int) string {
	q := url.Values{}
	q.Set("address", address)
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	auth := strings.TrimSpace(c.Query("auth"))
	if auth != "" {
		q.Set("auth", auth)
	}
	return "?" + q.Encode()
}
