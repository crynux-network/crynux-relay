package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NodeCapabilitiesInput struct {
	Address  string   `json:"address" path:"address" description:"address" validate:"required"`
	GPUName  string   `json:"gpu_name" description:"gpu_name" validate:"required"`
	GPUVram  uint64   `json:"gpu_vram" description:"gpu_vram" validate:"required"`
	Version  string   `json:"version" description:"version" validate:"required"`
	ModelIDs []string `json:"model_ids" description:"complete node local model ids"`
}

type NodeCapabilitiesInputWithSignature struct {
	NodeCapabilitiesInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func SyncNodeCapabilities(c *gin.Context, in *NodeCapabilitiesInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.NodeCapabilitiesInput, in.Timestamp, in.Signature)
	if err != nil || !match {
		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}
		return nil, response.NewValidationErrorResponse("signature", "Invalid signature")
	}
	if address != in.Address {
		return nil, response.NewValidationErrorResponse("address", "Signer not allowed")
	}
	if in.ModelIDs == nil {
		return nil, response.NewValidationErrorResponse("model_ids", "required")
	}

	major, minor, patch, err := service.ParseNodeVersion(in.Version)
	if err != nil {
		return nil, response.NewValidationErrorResponse("version", "Invalid node version")
	}

	err = service.SyncNodeCapabilities(c.Request.Context(), config.GetDB(), in.Address, service.NodeCapabilities{
		GPUName:      models.NormalizeGPUName(in.GPUName),
		GPUVram:      in.GPUVram,
		MajorVersion: major,
		MinorVersion: minor,
		PatchVersion: patch,
		ModelIDs:     models.NormalizeModelIDs(in.ModelIDs),
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, response.NewValidationErrorResponse("address", "Node not found")
	}
	if errors.Is(err, service.ErrNodeCapabilitiesIllegalStatus) {
		return nil, response.NewValidationErrorResponse("address", "Illegal node status")
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
