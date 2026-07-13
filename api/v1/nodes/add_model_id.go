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

type AddModelIDInput struct {
	Address string `json:"address" path:"address" description:"address" validate:"required"`
	ModelID string `json:"model_id" description:"new local model ID" validate:"required"`
}

type AddModelIDInputWithSignature struct {
	AddModelIDInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func AddModelID(c *gin.Context, in *AddModelIDInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.AddModelIDInput, in.Timestamp, in.Signature)

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

	if node.Status == models.NodeStatusQuit {
		return nil, response.NewValidationErrorResponse("address", "Illegal node status")
	}

	in.ModelID = models.NormalizeModelID(in.ModelID)

	err = service.ExecuteNodeStateUpdate(c.Request.Context(), config.GetDB(), []string{in.Address}, func() error {
		_, err := models.GetNodeModel(c.Request.Context(), config.GetDB(), in.Address, in.ModelID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nodeModel := models.NewNodeModel(in.Address, in.ModelID, false)
			return nodeModel.Save(c.Request.Context(), config.GetDB())
		}
		return err
	})
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
