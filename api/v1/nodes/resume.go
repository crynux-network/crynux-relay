package nodes

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ResumeInput struct {
	Address string `path:"address" json:"address" description:"address" validate:"required"`
}

type ResumeInputWithSignature struct {
	ResumeInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func NodeResume(c *gin.Context, in *ResumeInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.ResumeInput, in.Timestamp, in.Signature)

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

	err = service.ExecuteNodeStateUpdate(c.Request.Context(), config.GetDB(), []string{in.Address}, func() error {
		var err error
		for range 3 {
			if node.Status != models.NodeStatusPaused {
				return response.NewValidationErrorResponse("address", "Illegal node status")
			}

			err = config.GetDB().Transaction(func(tx *gorm.DB) error {
				if err := node.Update(c.Request.Context(), tx, map[string]interface{}{"status": models.NodeStatusAvailable}); err != nil {
					return err
				}
				return service.IncrementNodeNameCountTx(c.Request.Context(), tx, node)
			})
			if err == nil {
				service.ApplyNodeNameCountDeltaToCache(node.GPUName, node.GPUVram, service.BuildNodeVersion(node.MajorVersion, node.MinorVersion, node.PatchVersion), 1)
				service.LogNodeStatusChange(node, "resume")
				return nil
			} else if errors.Is(err, models.ErrNodeStatusChanged) {
				if err := node.SyncStatus(c.Request.Context(), config.GetDB()); err != nil {
					return response.NewExceptionResponse(err)
				}
			} else {
				return response.NewExceptionResponse(err)
			}
		}
		return response.NewExceptionResponse(err)
	})
	if err != nil {
		return nil, err
	}
	return &response.Response{}, nil
}
