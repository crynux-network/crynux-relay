package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"encoding/csv"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ListDelegatedSlashAuditsInput struct {
	NodeAddress      string `query:"node_address" validate:"required"`
	DelegatorAddress string `query:"delegator_address"`
}

type TriggerNodeSlashInput struct {
	NodeAddress    string `json:"node_address"`
	PendingSlashID uint   `json:"pending_slash_id"`
}

type TriggerNodeSlashData struct {
	BlockchainTransactionID uint `json:"blockchain_transaction_id"`
}

type TriggerNodeSlashResponse struct {
	response.Response
	Data TriggerNodeSlashData `json:"data"`
}

func ExportDelegatedSlashAuditsCSV(c *gin.Context) {
	input := ListDelegatedSlashAuditsInput{
		NodeAddress:      c.Query("node_address"),
		DelegatorAddress: c.Query("delegator_address"),
	}

	nodeAddress, node, ok := loadDelegatedSlashAuditNode(c, input.NodeAddress)
	if !ok {
		return
	}

	delegatorAddress := ""
	if input.DelegatorAddress != "" {
		if !common.IsHexAddress(input.DelegatorAddress) {
			c.JSON(400, gin.H{"message": "invalid delegator address"})
			return
		}
		delegatorAddress = common.HexToAddress(input.DelegatorAddress).Hex()
	}

	db := config.GetDB()
	query := db.Model(&models.DelegatedStakingSlashRecord{}).
		Where("node_address = ?", nodeAddress).
		Where("network = ?", node.Network)
	if delegatorAddress != "" {
		query = query.Where("delegator_address = ?", delegatorAddress)
	}

	var audits []models.DelegatedStakingSlashRecord
	if err := query.Order("block_number DESC, log_index DESC").
		Find(&audits).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=delegated_slash_audits_%s.csv", nodeAddress))

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{"node_address", "delegator_address", "network", "amount", "slash_tx_hash", "block_number", "log_index", "created_at", "updated_at"}); err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
	for _, audit := range audits {
		if err := writer.Write([]string{
			audit.NodeAddress,
			audit.DelegatorAddress,
			audit.Network,
			audit.Amount.String(),
			audit.SlashTxHash,
			fmt.Sprintf("%d", audit.BlockNumber),
			fmt.Sprintf("%d", audit.LogIndex),
			audit.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			audit.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}
}

func TriggerNodeSlash(c *gin.Context, in *TriggerNodeSlashInput) (*TriggerNodeSlashResponse, error) {
	var evidence *models.SlashEvidence
	taskIDCommitment := ""
	nodeAddressInput := in.NodeAddress
	var pendingSlash *models.PendingSlash
	if in.PendingSlashID != 0 {
		var err error
		pendingSlash, err = models.GetPendingSlashByID(c.Request.Context(), config.GetDB(), in.PendingSlashID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, response.NewNotFoundErrorResponse()
			}
			return nil, response.NewExceptionResponse(err)
		}
		if pendingSlash.Status != models.PendingSlashStatusPending {
			return nil, &response.ErrorResponse{Response: response.Response{Message: "pending slash is not pending"}}
		}
		nodeAddressInput = pendingSlash.NodeAddress
		taskIDCommitment = pendingSlash.TaskIDCommitment
		evidence, err = service.ParsePendingSlashEvidence(pendingSlash)
		if err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	} else if !common.IsHexAddress(nodeAddressInput) {
		return nil, &response.ErrorResponse{Response: response.Response{Message: "invalid node address"}}
	}

	_, node, err := getNodeByAdminAddress(c, nodeAddressInput)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.ErrorResponse{Response: response.Response{Message: "node not found"}}
		}
		return nil, response.NewExceptionResponse(err)
	}

	blockchainTransactionID, err := service.SlashNode(c.Request.Context(), config.GetDB(), node, taskIDCommitment, evidence)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if pendingSlash != nil {
		pendingSlash.Status = models.PendingSlashStatusSlashed
		if err := pendingSlash.Save(c.Request.Context(), config.GetDB()); err != nil {
			return nil, response.NewExceptionResponse(err)
		}
	}

	return &TriggerNodeSlashResponse{
		Data: TriggerNodeSlashData{
			BlockchainTransactionID: blockchainTransactionID,
		},
	}, nil
}

func loadDelegatedSlashAuditNode(c *gin.Context, rawNodeAddress string) (string, *models.Node, bool) {
	nodeAddress, node, err := getNodeByAdminAddress(c, rawNodeAddress)
	if err == nil {
		return nodeAddress, node, true
	}
	if errors.Is(err, errInvalidAdminNodeAddress) {
		c.JSON(400, gin.H{"message": "invalid node address"})
		return "", nil, false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(400, gin.H{"message": "node not found"})
		return "", nil, false
	}
	c.JSON(500, gin.H{"message": err.Error()})
	return "", nil, false
}

var errInvalidAdminNodeAddress = errors.New("invalid node address")

func getNodeByAdminAddress(c *gin.Context, rawNodeAddress string) (string, *models.Node, error) {
	if !common.IsHexAddress(rawNodeAddress) {
		return "", nil, errInvalidAdminNodeAddress
	}
	nodeAddress := common.HexToAddress(rawNodeAddress).Hex()
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), nodeAddress)
	if err != nil {
		return "", nil, err
	}
	return nodeAddress, node, nil
}
