package admin

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultNodeTaskErrorPageSize = 30
	maxNodeTaskErrorPageSize     = 100
)

type ListNodeTaskErrorsInput struct {
	NodeAddress      string `query:"node_address"`
	TaskIDCommitment string `query:"task_id_commitment"`
	Page             int    `query:"page" default:"1"`
	PageSize         int    `query:"page_size" default:"30"`
}

type NodeTaskErrorRecord struct {
	ID               uint   `json:"id"`
	NodeAddress      string `json:"node_address"`
	TaskIDCommitment string `json:"task_id_commitment"`
	TaskArgs         string `json:"task_args"`
	ErrorType        string `json:"error_type"`
	Message          string `json:"message"`
	StackTrace       string `json:"stack_trace"`
	CapturedAt       int64  `json:"captured_at"`
	CreatedAt        int64  `json:"created_at"`
}

type ListNodeTaskErrorsData struct {
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Items    []NodeTaskErrorRecord `json:"items"`
}

type ListNodeTaskErrorsResponse struct {
	response.Response
	Data ListNodeTaskErrorsData `json:"data"`
}

func ListNodeTaskErrors(c *gin.Context, in *ListNodeTaskErrorsInput) (*ListNodeTaskErrorsResponse, error) {
	return listNodeTaskErrors(c.Request.Context(), config.GetDB(), in)
}

func listNodeTaskErrors(ctx context.Context, db *gorm.DB, in *ListNodeTaskErrorsInput) (*ListNodeTaskErrorsResponse, error) {
	page, pageSize := clampNodeTaskErrorPagination(in.Page, in.PageSize)
	records, total, err := service.ListNodeTaskErrors(
		ctx,
		db,
		service.NodeTaskErrorFilter{
			NodeAddress:      in.NodeAddress,
			TaskIDCommitment: in.TaskIDCommitment,
		},
		page,
		pageSize,
	)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	items := make([]NodeTaskErrorRecord, 0, len(records))
	for _, record := range records {
		items = append(items, buildNodeTaskErrorRecord(record))
	}
	return &ListNodeTaskErrorsResponse{
		Data: ListNodeTaskErrorsData{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			Items:    items,
		},
	}, nil
}

func clampNodeTaskErrorPagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultNodeTaskErrorPageSize
	}
	if pageSize > maxNodeTaskErrorPageSize {
		pageSize = maxNodeTaskErrorPageSize
	}
	return page, pageSize
}

func buildNodeTaskErrorRecord(record models.NodeTaskError) NodeTaskErrorRecord {
	return NodeTaskErrorRecord{
		ID:               record.ID,
		NodeAddress:      record.NodeAddress,
		TaskIDCommitment: record.TaskIDCommitment,
		TaskArgs:         record.TaskArgs,
		ErrorType:        record.ErrorType,
		Message:          record.Message,
		StackTrace:       record.StackTrace,
		CapturedAt:       record.CapturedAt,
		CreatedAt:        record.CreatedAt.Unix(),
	}
}
