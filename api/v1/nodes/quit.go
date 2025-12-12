package nodes

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type QuitInput struct {
	Address string `path:"address" json:"address" description:"address" validate:"required"`
}

type QuitInputWithSignature struct {
	QuitInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func NodeQuit(c *gin.Context, in *QuitInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.QuitInput, in.Timestamp, in.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}

		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	if in.Address != address {
		return nil, response.NewValidationErrorResponse("signature", "Signer not allowed")
	}

	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			validationErr := response.NewValidationErrorResponse("address", "Node not found")
			return nil, validationErr
		}
		return nil, response.NewExceptionResponse(err)
	}
	stakingInfo, err := blockchain.GetStakingInfo(c.Request.Context(), common.HexToAddress(in.Address), node.Network)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}

	if stakingInfo.Status == 1 { // staked
		return nil, response.NewValidationErrorResponse("staking_status", "Staking status is staked")
	}
retryLoop:
	for range 3 {
		switch node.Status {
		case models.NodeStatusAvailable, models.NodeStatusPaused:
			err = service.SetNodeStatusQuit(c.Request.Context(), config.GetDB(), node, false)
			if err == nil {
				break retryLoop
			} else if errors.Is(err, models.ErrNodeStatusChanged) {
				if err := node.SyncStatus(c.Request.Context(), config.GetDB()); err != nil {
					return nil, response.NewExceptionResponse(err)
				}
			} else {
				return nil, response.NewExceptionResponse(err)
			}
		case models.NodeStatusBusy:
			err = node.Update(c.Request.Context(), config.GetDB(), map[string]interface{}{"status": models.NodeStatusPendingQuit})
			if err == nil {
				break retryLoop
			} else if errors.Is(err, models.ErrNodeStatusChanged) {
				if err := node.SyncStatus(c.Request.Context(), config.GetDB()); err != nil {
					return nil, response.NewExceptionResponse(err)
				}
			} else {
				return nil, response.NewExceptionResponse(err)
			}
		default:
			return nil, response.NewValidationErrorResponse("address", "Illegal node status")
		}
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
