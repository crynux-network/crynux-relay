package admin

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultSlashReportPageSize = 30
	maxSlashReportPageSize     = 100
)

type ListSlashNodesInput struct {
	Network  string `query:"network"`
	Page     int    `query:"page" default:"1"`
	PageSize int    `query:"page_size" default:"30"`
}

type GetSlashEventInput struct {
	SlashEventID uint   `path:"slash_event_id" validate:"required"`
	Network      string `query:"network"`
}

type ListSlashNodeDelegatorsInput struct {
	Address      string `path:"address" validate:"required"`
	Network      string `query:"network"`
	SlashEventID uint   `query:"slash_event_id"`
	SlashJobID   uint   `query:"slash_job_id"`
	Page         int    `query:"page" default:"1"`
	PageSize     int    `query:"page_size" default:"30"`
}

type ListSlashNodeVestingsInput struct {
	Address  string `path:"address" validate:"required"`
	Page     int    `query:"page" default:"1"`
	PageSize int    `query:"page_size" default:"30"`
}

type SlashedNodeRecord struct {
	SlashEventID                 uint                  `json:"slash_event_id"`
	Address                      string                `json:"address"`
	CardName                     string                `json:"card_name"`
	Network                      string                `json:"network"`
	OperatorSlashedAmount        string                `json:"operator_slashed_amount"`
	TaskIDCommitment             string                `json:"task_id_commitment"`
	QueuedTransactionID          *uint                 `json:"queued_transaction_id"`
	ConfirmedOperatorSlashTxHash string                `json:"confirmed_operator_slash_tx_hash"`
	DelegatedSlashJobID          *uint                 `json:"delegated_slash_job_id"`
	DelegatedSlashJobStatus      string                `json:"delegated_slash_job_status"`
	Evidence                     *models.SlashEvidence `json:"evidence,omitempty"`
	CreatedAt                    int64                 `json:"created_at"`
}

type DelegatedSlashAuditRecord struct {
	ID               uint   `json:"id"`
	SlashJobID       *uint  `json:"slash_job_id"`
	NodeAddress      string `json:"node_address"`
	DelegatorAddress string `json:"delegator_address"`
	Network          string `json:"network"`
	Amount           string `json:"amount"`
	SlashTxHash      string `json:"slash_tx_hash"`
	BlockNumber      uint64 `json:"block_number"`
	LogIndex         uint   `json:"log_index"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type SlashVestingRecord struct {
	ID             uint                 `json:"id"`
	Address        string               `json:"address"`
	Amount         string               `json:"amount"`
	LockedAmount   string               `json:"locked_amount"`
	ReleasedAmount string               `json:"released_amount"`
	Slashed        bool                 `json:"slashed"`
	Source         string               `json:"source"`
	ExternalID     string               `json:"external_id"`
	Status         models.VestingStatus `json:"status"`
	Type           string               `json:"type"`
	CreatedAt      int64                `json:"created_at"`
	StartTime      int64                `json:"start_time"`
	DurationDays   uint                 `json:"duration_days"`
}

type ListSlashNodesData struct {
	Total  int64               `json:"total"`
	Events []SlashedNodeRecord `json:"events"`
}

type ListSlashNodesResponse struct {
	response.Response
	Data ListSlashNodesData `json:"data"`
}

type ListSlashNodeDelegatorsData struct {
	Total   int64                       `json:"total"`
	Records []DelegatedSlashAuditRecord `json:"records"`
}

type ListSlashNodeDelegatorsResponse struct {
	response.Response
	Data ListSlashNodeDelegatorsData `json:"data"`
}

type ListSlashNodeVestingsData struct {
	Total   int64                `json:"total"`
	Records []SlashVestingRecord `json:"records"`
}

type ListSlashNodeVestingsResponse struct {
	response.Response
	Data ListSlashNodeVestingsData `json:"data"`
}

type SlashEventDetailData struct {
	Event                 SlashedNodeRecord           `json:"event"`
	DelegatedSlashRecords []DelegatedSlashAuditRecord `json:"delegated_slash_records"`
	DelegatedSlashTotal   int64                       `json:"delegated_slash_total"`
	VestingRecords        []SlashVestingRecord        `json:"vesting_records"`
	VestingTotal          int64                       `json:"vesting_total"`
}

type GetSlashEventResponse struct {
	response.Response
	Data SlashEventDetailData `json:"data"`
}

func ListSlashNodes(c *gin.Context, in *ListSlashNodesInput) (*ListSlashNodesResponse, error) {
	page, pageSize := clampSlashReportPagination(in.Page, in.PageSize)
	records, total, err := querySlashedNodeRecords(c.Request.Context(), config.GetDB(), in.Network, page, pageSize)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListSlashNodesResponse{
		Data: ListSlashNodesData{
			Total:  total,
			Events: records,
		},
	}, nil
}

func GetSlashEvent(c *gin.Context, in *GetSlashEventInput) (*GetSlashEventResponse, error) {
	record, err := querySlashedNodeRecordByID(c.Request.Context(), config.GetDB(), in.SlashEventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}
	if in.Network != "" && record.Network != in.Network {
		return nil, response.NewNotFoundErrorResponse()
	}

	var slashJobID *uint
	if record.DelegatedSlashJobID != nil {
		slashJobID = record.DelegatedSlashJobID
	}
	var delegators []DelegatedSlashAuditRecord
	var delegatorTotal int64
	if slashJobID != nil {
		var err error
		delegators, delegatorTotal, err = queryDelegatedSlashAuditRecords(c.Request.Context(), config.GetDB(), record.Address, record.Network, slashJobID, 1, maxSlashReportPageSize)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}
	vestings, vestingTotal, err := querySlashVestingRecords(c.Request.Context(), config.GetDB(), record.Address, 1, maxSlashReportPageSize, time.Now().UTC())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &GetSlashEventResponse{
		Data: SlashEventDetailData{
			Event:                 *record,
			DelegatedSlashRecords: delegators,
			DelegatedSlashTotal:   delegatorTotal,
			VestingRecords:        vestings,
			VestingTotal:          vestingTotal,
		},
	}, nil
}

func ListSlashNodeDelegators(c *gin.Context, in *ListSlashNodeDelegatorsInput) (*ListSlashNodeDelegatorsResponse, error) {
	nodeAddress, node, err := getNodeByAdminAddress(c, in.Address)
	if err != nil {
		if errors.Is(err, errInvalidAdminNodeAddress) {
			return nil, &response.ErrorResponse{Response: response.Response{Message: "invalid node address"}}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}

	network := node.Network
	if in.Network != "" {
		network = in.Network
	}

	var slashJobID *uint
	if in.SlashJobID != 0 {
		slashJobID = &in.SlashJobID
	}
	if in.SlashEventID != 0 {
		eventRecord, err := querySlashedNodeRecordByID(c.Request.Context(), config.GetDB(), in.SlashEventID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, response.NewNotFoundErrorResponse()
			}
			return nil, response.NewExceptionResponse(err)
		}
		if eventRecord.Address != nodeAddress || eventRecord.Network != network {
			return nil, response.NewNotFoundErrorResponse()
		}
		slashJobID = eventRecord.DelegatedSlashJobID
		if slashJobID == nil {
			return &ListSlashNodeDelegatorsResponse{
				Data: ListSlashNodeDelegatorsData{
					Records: []DelegatedSlashAuditRecord{},
				},
			}, nil
		}
	}

	page, pageSize := clampSlashReportPagination(in.Page, in.PageSize)
	records, total, err := queryDelegatedSlashAuditRecords(c.Request.Context(), config.GetDB(), nodeAddress, network, slashJobID, page, pageSize)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListSlashNodeDelegatorsResponse{
		Data: ListSlashNodeDelegatorsData{
			Total:   total,
			Records: records,
		},
	}, nil
}

func ListSlashNodeVestings(c *gin.Context, in *ListSlashNodeVestingsInput) (*ListSlashNodeVestingsResponse, error) {
	nodeAddress, _, err := getNodeByAdminAddress(c, in.Address)
	if err != nil {
		if errors.Is(err, errInvalidAdminNodeAddress) {
			return nil, &response.ErrorResponse{Response: response.Response{Message: "invalid node address"}}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}

	page, pageSize := clampSlashReportPagination(in.Page, in.PageSize)
	records, total, err := querySlashVestingRecords(c.Request.Context(), config.GetDB(), nodeAddress, page, pageSize, time.Now().UTC())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListSlashNodeVestingsResponse{
		Data: ListSlashNodeVestingsData{
			Total:   total,
			Records: records,
		},
	}, nil
}

func clampSlashReportPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultSlashReportPageSize
	}
	if pageSize > maxSlashReportPageSize {
		pageSize = maxSlashReportPageSize
	}
	return page, pageSize
}

func querySlashedNodeRecords(ctx context.Context, db *gorm.DB, network string, page, pageSize int) ([]SlashedNodeRecord, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var events []models.Event
	if err := db.WithContext(dbCtx).
		Model(&models.Event{}).
		Where("type = ?", "NodeSlashed").
		Order("created_at DESC, id DESC").
		Find(&events).Error; err != nil {
		return nil, 0, err
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	records := make([]SlashedNodeRecord, 0, pageSize)
	var total int64
	for _, event := range events {
		record, err := buildSlashedNodeRecord(dbCtx, db, event)
		if err != nil {
			return nil, 0, err
		}
		if network != "" && record.Network != network {
			continue
		}
		if int(total) >= start && int(total) < end {
			records = append(records, *record)
		}
		total++
	}

	return records, total, nil
}

func querySlashedNodeRecordByID(ctx context.Context, db *gorm.DB, slashEventID uint) (*SlashedNodeRecord, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var event models.Event
	if err := db.WithContext(dbCtx).
		Model(&models.Event{}).
		Where("id = ? AND type = ?", slashEventID, "NodeSlashed").
		First(&event).Error; err != nil {
		return nil, err
	}
	return buildSlashedNodeRecord(dbCtx, db, event)
}

func buildSlashedNodeRecord(ctx context.Context, db *gorm.DB, event models.Event) (*SlashedNodeRecord, error) {
	var eventArgs models.NodeSlashedEvent
	if err := json.Unmarshal([]byte(event.Args), &eventArgs); err != nil {
		return nil, err
	}

	record := &SlashedNodeRecord{
		SlashEventID:          event.ID,
		Address:               event.NodeAddress,
		Network:               eventArgs.Network,
		OperatorSlashedAmount: eventArgs.Amount.String(),
		TaskIDCommitment:      event.TaskIDCommitment,
		Evidence:              eventArgs.Evidence,
		CreatedAt:             event.CreatedAt.Unix(),
	}
	if record.TaskIDCommitment == "" {
		record.TaskIDCommitment = eventArgs.TaskIDCommitment
	}
	if eventArgs.NodeAddress != "" {
		record.Address = eventArgs.NodeAddress
	}
	if eventArgs.Evidence != nil {
		if nodeSnapshot := findSlashEvidenceNodeSnapshot(eventArgs.Evidence, record.Address); nodeSnapshot != nil {
			record.CardName = nodeSnapshot.GPUName
		}
		if taskSnapshot := findSlashEvidenceTaskSnapshot(eventArgs.Evidence, record.TaskIDCommitment); taskSnapshot != nil {
			record.TaskIDCommitment = taskSnapshot.TaskIDCommitment
		}
	}

	if record.CardName == "" {
		var node models.Node
		if err := db.WithContext(ctx).
			Model(&models.Node{}).
			Where("address = ?", record.Address).
			First(&node).Error; err == nil {
			record.CardName = node.GPUName
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	queuedTxID, txHash, err := findOperatorSlashTransaction(ctx, db, event.ID, record.Address, record.Network)
	if err != nil {
		return nil, err
	}
	record.QueuedTransactionID = queuedTxID
	record.ConfirmedOperatorSlashTxHash = txHash

	if txHash != "" {
		job, err := findDelegatedSlashJobByNodeSlashTx(ctx, db, record.Address, record.Network, txHash)
		if err != nil {
			return nil, err
		}
		if job != nil {
			record.DelegatedSlashJobID = &job.ID
			record.DelegatedSlashJobStatus = string(job.Status)
		}
	}

	return record, nil
}

func findSlashEvidenceNodeSnapshot(evidence *models.SlashEvidence, nodeAddress string) *models.SlashEvidenceNodeSnapshot {
	for i := range evidence.NodeSnapshots {
		if evidence.NodeSnapshots[i].Address == nodeAddress {
			return &evidence.NodeSnapshots[i]
		}
	}
	if len(evidence.NodeSnapshots) == 0 {
		return nil
	}
	return &evidence.NodeSnapshots[0]
}

func findSlashEvidenceTaskSnapshot(evidence *models.SlashEvidence, taskIDCommitment string) *models.SlashEvidenceTaskSnapshot {
	for i := range evidence.TaskSnapshots {
		if evidence.TaskSnapshots[i].TaskIDCommitment == taskIDCommitment {
			return &evidence.TaskSnapshots[i]
		}
	}
	if len(evidence.TaskSnapshots) == 0 {
		return nil
	}
	return &evidence.TaskSnapshots[0]
}

func findOperatorSlashTransaction(ctx context.Context, db *gorm.DB, slashEventID uint, nodeAddress, network string) (*uint, string, error) {
	var quitEvents []models.Event
	if err := db.WithContext(ctx).
		Model(&models.Event{}).
		Where("type = ? AND node_address = ? AND id < ?", "NodeQuit", nodeAddress, slashEventID).
		Order("id DESC").
		Limit(20).
		Find(&quitEvents).Error; err != nil {
		return nil, "", err
	}

	for _, quitEvent := range quitEvents {
		var quitArgs models.NodeQuitEvent
		if err := json.Unmarshal([]byte(quitEvent.Args), &quitArgs); err != nil {
			return nil, "", err
		}
		if quitArgs.Network != network {
			continue
		}
		if quitArgs.BlockchainTransactionID == 0 {
			return nil, "", nil
		}
		txID := quitArgs.BlockchainTransactionID
		var tx models.BlockchainTransaction
		if err := db.WithContext(ctx).First(&tx, txID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &txID, "", nil
			}
			return nil, "", err
		}
		if tx.TxHash.Valid {
			return &txID, tx.TxHash.String, nil
		}
		return &txID, "", nil
	}

	return nil, "", nil
}

func findDelegatedSlashJobByNodeSlashTx(ctx context.Context, db *gorm.DB, nodeAddress, network, txHash string) (*models.DelegatedSlashJob, error) {
	var job models.DelegatedSlashJob
	if err := db.WithContext(ctx).
		Model(&models.DelegatedSlashJob{}).
		Where("node_address = ? AND network = ? AND node_slash_tx_hash = ?", nodeAddress, network, txHash).
		First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func queryDelegatedSlashAuditRecords(ctx context.Context, db *gorm.DB, nodeAddress, network string, slashJobID *uint, page, pageSize int) ([]DelegatedSlashAuditRecord, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := db.WithContext(dbCtx).
		Model(&models.DelegatedStakingSlashRecord{}).
		Where("node_address = ? AND network = ?", nodeAddress, network)
	if slashJobID != nil {
		query = query.Where("slash_job_id = ?", *slashJobID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []models.DelegatedStakingSlashRecord
	if err := query.
		Order("block_number DESC, log_index DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}

	results := make([]DelegatedSlashAuditRecord, 0, len(records))
	for _, record := range records {
		results = append(results, buildDelegatedSlashAuditRecord(record))
	}
	return results, total, nil
}

func buildDelegatedSlashAuditRecord(record models.DelegatedStakingSlashRecord) DelegatedSlashAuditRecord {
	return DelegatedSlashAuditRecord{
		ID:               record.ID,
		SlashJobID:       uintPtrFromNullInt64(record.SlashJobID),
		NodeAddress:      record.NodeAddress,
		DelegatorAddress: record.DelegatorAddress,
		Network:          record.Network,
		Amount:           record.Amount.String(),
		SlashTxHash:      record.SlashTxHash,
		BlockNumber:      record.BlockNumber,
		LogIndex:         record.LogIndex,
		CreatedAt:        record.CreatedAt.Unix(),
		UpdatedAt:        record.UpdatedAt.Unix(),
	}
}

func querySlashVestingRecords(ctx context.Context, db *gorm.DB, nodeAddress string, page, pageSize int, now time.Time) ([]SlashVestingRecord, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := db.WithContext(dbCtx).
		Model(&models.VestingRecord{}).
		Where("address = ?", nodeAddress)

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

	results := make([]SlashVestingRecord, 0, len(records))
	for _, record := range records {
		results = append(results, buildSlashVestingRecord(record, now))
	}
	return results, total, nil
}

func buildSlashVestingRecord(record models.VestingRecord, now time.Time) SlashVestingRecord {
	lockedAmount := record.LockedAmountAt(now)
	if record.Slashed {
		lockedAmount = big.NewInt(0)
	}

	return SlashVestingRecord{
		ID:             record.ID,
		Address:        record.Address,
		Amount:         record.TotalAmount.String(),
		LockedAmount:   lockedAmount.String(),
		ReleasedAmount: record.ReleasedAmount.String(),
		Slashed:        record.Slashed,
		Source:         record.Source,
		ExternalID:     record.ExternalID,
		Status:         record.Status,
		Type:           record.Type,
		CreatedAt:      record.CreatedAt.Unix(),
		StartTime:      record.StartTime.Unix(),
		DurationDays:   record.DurationDays,
	}
}

func uintPtrFromNullInt64(value sql.NullInt64) *uint {
	if !value.Valid {
		return nil
	}
	result := uint(value.Int64)
	return &result
}
