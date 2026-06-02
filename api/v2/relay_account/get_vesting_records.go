package relayaccount

import (
	"context"
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultVestingPageSize = 20
	maxVestingPageSize     = 100
)

type GetVestingRecordsInput struct {
	Address  string `path:"address" description:"The address of the user"`
	Page     int    `query:"page" description:"The page number" default:"1"`
	PageSize int    `query:"page_size" description:"The page size" default:"20"`
}

type VestingRecord struct {
	ID              uint                 `json:"id"`
	CreatedAt       uint64               `json:"created_at"`
	Address         string               `json:"address"`
	TotalAmount     string               `json:"total_amount"`
	StartTime       int64                `json:"start_time"`
	DurationDays    uint                 `json:"duration_days"`
	ReleasedAmount  string               `json:"released_amount"`
	RemainingAmount string               `json:"remaining_amount"`
	LockedAmount    string               `json:"locked_amount"`
	Status          models.VestingStatus `json:"status"`
	Source          string               `json:"source"`
	ExternalID      string               `json:"external_id"`
}

type GetVestingRecordsData struct {
	Total          int64           `json:"total" description:"The total number of vesting records"`
	VestingRecords []VestingRecord `json:"vesting_records" description:"The vesting records"`
}

type GetVestingRecordsResponse struct {
	response.Response
	Data *GetVestingRecordsData `json:"data" description:"The data of the vesting records"`
}

func clampVestingPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultVestingPageSize
	}
	if pageSize > maxVestingPageSize {
		pageSize = maxVestingPageSize
	}
	return page, pageSize
}

func buildVestingRecord(record models.VestingRecord, now time.Time) VestingRecord {
	remainingAmount := big.NewInt(0).Sub(&record.TotalAmount.Int, &record.ReleasedAmount.Int)
	if remainingAmount.Sign() < 0 {
		remainingAmount = big.NewInt(0)
	}

	return VestingRecord{
		ID:              record.ID,
		CreatedAt:       uint64(record.CreatedAt.Unix()),
		Address:         record.Address,
		TotalAmount:     record.TotalAmount.String(),
		StartTime:       record.StartTime.Unix(),
		DurationDays:    record.DurationDays,
		ReleasedAmount:  record.ReleasedAmount.String(),
		RemainingAmount: remainingAmount.String(),
		LockedAmount:    record.LockedAmountAt(now).String(),
		Status:          record.Status,
		Source:          record.Source,
		ExternalID:      record.ExternalID,
	}
}

func queryAddressVestingRecords(ctx context.Context, db *gorm.DB, dbAddress string, page, pageSize int, now time.Time) ([]VestingRecord, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := db.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Where("address = ?", dbAddress)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []models.VestingRecord
	if err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	results := make([]VestingRecord, 0, len(records))
	for _, record := range records {
		results = append(results, buildVestingRecord(record, now))
	}
	return results, total, nil
}

func GetVestingRecords(c *gin.Context, in *GetVestingRecordsInput) (*GetVestingRecordsResponse, error) {
	address := middleware.GetUserAddress(c)
	if address != in.Address {
		return nil, response.NewValidationErrorResponse("address", "Address mismatch")
	}

	page, pageSize := clampVestingPagination(in.Page, in.PageSize)
	records, total, err := queryAddressVestingRecords(c.Request.Context(), config.GetDB(), in.Address, page, pageSize, time.Now().UTC())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &GetVestingRecordsResponse{
		Data: &GetVestingRecordsData{
			Total:          total,
			VestingRecords: records,
		},
	}, nil
}
