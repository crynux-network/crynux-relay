package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ListDelegatedSlashAuditsInput struct {
	NodeAddress      string `query:"node_address" validate:"required"`
	Network          string `query:"network" validate:"required"`
	DelegatorAddress string `query:"delegator_address"`
	Page             int    `query:"page"`
	PageSize         int    `query:"page_size"`
}

type DelegatedSlashAuditRecord struct {
	NodeAddress      string `json:"node_address"`
	DelegatorAddress string `json:"delegator_address"`
	Network          string `json:"network"`
	Amount           string `json:"amount"`
	SlashTxHash      string `json:"slash_tx_hash"`
	BlockNumber      uint64 `json:"block_number"`
	LogIndex         uint   `json:"log_index"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type ListDelegatedSlashAuditsData struct {
	Total   int64                       `json:"total"`
	Records []DelegatedSlashAuditRecord `json:"records"`
}

type ListDelegatedSlashAuditsResponse struct {
	response.Response
	Data ListDelegatedSlashAuditsData `json:"data"`
}

type TriggerNodeSlashInput struct {
	NodeAddress string `json:"node_address" validate:"required"`
}

type TriggerNodeSlashData struct {
	BlockchainTransactionID uint `json:"blockchain_transaction_id"`
}

type TriggerNodeSlashResponse struct {
	response.Response
	Data TriggerNodeSlashData `json:"data"`
}

func ListDelegatedSlashAudits(c *gin.Context, in *ListDelegatedSlashAuditsInput) (*ListDelegatedSlashAuditsResponse, error) {
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 30
	}
	if pageSize > 100 {
		pageSize = 100
	}

	db := config.GetDB().WithContext(c.Request.Context())
	query := db.Model(&models.DelegatedStakingSlashRecord{}).
		Where("node_address = ?", in.NodeAddress).
		Where("network = ?", in.Network)
	if in.DelegatorAddress != "" {
		query = query.Where("delegator_address = ?", in.DelegatorAddress)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	var audits []models.DelegatedStakingSlashRecord
	if err := query.Order("block_number DESC, log_index DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&audits).Error; err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	records := make([]DelegatedSlashAuditRecord, 0, len(audits))
	for _, audit := range audits {
		records = append(records, DelegatedSlashAuditRecord{
			NodeAddress:      audit.NodeAddress,
			DelegatorAddress: audit.DelegatorAddress,
			Network:          audit.Network,
			Amount:           audit.Amount.String(),
			SlashTxHash:      audit.SlashTxHash,
			BlockNumber:      audit.BlockNumber,
			LogIndex:         audit.LogIndex,
			CreatedAt:        audit.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:        audit.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &ListDelegatedSlashAuditsResponse{
		Data: ListDelegatedSlashAuditsData{
			Total:   total,
			Records: records,
		},
	}, nil
}

func TriggerNodeSlash(c *gin.Context, in *TriggerNodeSlashInput) (*TriggerNodeSlashResponse, error) {
	if !common.IsHexAddress(in.NodeAddress) {
		return nil, &response.ErrorResponse{Response: response.Response{Message: "invalid node address"}}
	}

	nodeAddress := common.HexToAddress(in.NodeAddress).Hex()
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), nodeAddress)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.ErrorResponse{Response: response.Response{Message: "node not found"}}
		}
		return nil, response.NewExceptionResponse(err)
	}

	var blockchainTransaction *models.BlockchainTransaction
	err = config.GetDB().WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		blockchainTransaction, err = blockchain.QueueSlashStaking(c.Request.Context(), tx, common.HexToAddress(nodeAddress), node.Network)
		if err != nil {
			return err
		}
		_, err = service.SlashNodeVestingsTx(c.Request.Context(), tx, nodeAddress)
		return err
	})
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if err := service.RefreshNodeScoreStake(c.Request.Context(), config.GetDB(), nodeAddress, time.Now().UTC()); err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	return &TriggerNodeSlashResponse{
		Data: TriggerNodeSlashData{
			BlockchainTransactionID: blockchainTransaction.ID,
		},
	}, nil
}
