package nodes

import (
	"context"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type UpdateVersionInput struct {
	Address string `json:"address" path:"address" description:"address" validate:"required"`
	Version string `json:"version" description:"new node version" validate:"required"`
}

type UpdateVersionInputWithSignature struct {
	UpdateVersionInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func UpdateNodeVersion(c *gin.Context, in *UpdateVersionInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.UpdateVersionInput, in.Timestamp, in.Signature)

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

	versions := strings.Split(in.Version, ".")
	if len(versions) != 3 {
		return nil, response.NewValidationErrorResponse("version", "Invalid node version")
	}
	nodeVersions := make([]uint64, 3)
	for i := 0; i < 3; i++ {
		if v, err := strconv.ParseUint(versions[i], 10, 64); err != nil {
			return nil, response.NewValidationErrorResponse("version", "Invalid node version")
		} else {
			nodeVersions[i] = v
		}
	}

	var rowsAffected int64
	if err := service.ExecuteNodeStateUpdate(c.Request.Context(), config.GetDB(), []string{in.Address}, func() error {
		dbCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		res := config.GetDB().WithContext(dbCtx).Model(&models.Node{}).Where("address = ?", in.Address).Updates(map[string]interface{}{"major_version": nodeVersions[0], "minor_version": nodeVersions[1], "patch_version": nodeVersions[2]})
		if res.Error != nil {
			return res.Error
		}
		rowsAffected = res.RowsAffected
		return nil
	}); err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if rowsAffected == 0 {
		return nil, response.NewValidationErrorResponse("address", "Node not found")
	}

	return &response.Response{}, nil
}
