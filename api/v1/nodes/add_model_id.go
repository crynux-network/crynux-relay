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
	Address   string                `json:"address" path:"address" description:"address" validate:"required"`
	ModelFile models.ModelFileEntry `json:"model_file" description:"new local model file with a concrete revision" validate:"required"`
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

	files, err := service.BuildHeldModelFiles([]models.ModelFileEntry{in.ModelFile})
	if err != nil {
		return nil, response.NewValidationErrorResponse("model_file", err.Error())
	}

	if err := service.AddNodeModelFile(c.Request.Context(), config.GetDB(), in.Address, files[0]); err != nil {
		if errors.Is(err, service.ErrNodeModelStateIllegal) {
			return nil, response.NewValidationErrorResponse("address", "Illegal node status")
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
