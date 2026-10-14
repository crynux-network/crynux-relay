package nodes

import (
	"bytes"
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// InMemoryModel is the model a node currently holds in memory.
type InMemoryModel struct {
	ModelCacheHash   string                  `json:"model_cache_hash" description:"hex SHA-256 of the canonical model cache config" validate:"required"`
	ModelCacheConfig json.RawMessage         `json:"model_cache_config" description:"model cache config that the hash is computed from" validate:"required"`
	ModelFiles       []models.ModelFileEntry `json:"model_files" description:"exact model files of the in-memory model" validate:"required"`
	ExecutionDType   string                  `json:"execution_dtype" description:"dtype the model is loaded with" validate:"required"`
}

type NodeInMemoryModelInput struct {
	Address string         `json:"address" path:"address" description:"address" validate:"required"`
	Model   *InMemoryModel `json:"model" description:"current in-memory model; null when the node memory is empty"`
}

type NodeInMemoryModelInputWithSignature struct {
	NodeInMemoryModelInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func UpdateNodeInMemoryModel(c *gin.Context, in *NodeInMemoryModelInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.NodeInMemoryModelInput, in.Timestamp, in.Signature)
	if err != nil || !match {
		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}
		return nil, response.NewValidationErrorResponse("signature", "Invalid signature")
	}
	if address != in.Address {
		return nil, response.NewValidationErrorResponse("address", "Signer not allowed")
	}

	var report *service.NodeInMemoryModelReport
	if in.Model != nil {
		report, err = buildInMemoryModelReport(in.Model)
		if err != nil {
			return nil, response.NewValidationErrorResponse("model", err.Error())
		}
	}

	err = service.SetNodeInMemoryModel(c.Request.Context(), config.GetDB(), in.Address, report)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, response.NewValidationErrorResponse("address", "Node not found")
	}
	if errors.Is(err, service.ErrNodeModelStateIllegal) {
		return nil, response.NewValidationErrorResponse("address", "Illegal node status")
	}
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}

// buildInMemoryModelReport validates the reported model and checks that the
// cache hash is the hash of the reported cache config.
func buildInMemoryModelReport(model *InMemoryModel) (*service.NodeInMemoryModelReport, error) {
	hashBytes, err := hex.DecodeString(model.ModelCacheHash)
	if err != nil {
		return nil, errors.New("model_cache_hash is not hex")
	}
	cacheHash, err := models.Hash32FromBytes(hashBytes)
	if err != nil {
		return nil, errors.New("model_cache_hash must be 32 bytes")
	}

	decoder := json.NewDecoder(bytes.NewReader(model.ModelCacheConfig))
	decoder.UseNumber()
	var cacheConfig any
	if err := decoder.Decode(&cacheConfig); err != nil {
		return nil, errors.New("model_cache_config is not valid JSON")
	}
	canonicalConfig, err := models.CanonicalJSON(cacheConfig)
	if err != nil {
		return nil, err
	}
	if models.Hash32(sha256.Sum256(canonicalConfig)) != cacheHash {
		return nil, errors.New("model_cache_hash does not match model_cache_config")
	}

	if len(model.ModelFiles) == 0 {
		return nil, errors.New("model_files is empty")
	}
	files, err := service.BuildHeldModelFiles(model.ModelFiles)
	if err != nil {
		return nil, err
	}
	return &service.NodeInMemoryModelReport{
		CacheHash:      cacheHash,
		Files:          files,
		CacheConfig:    canonicalConfig,
		ExecutionDType: model.ExecutionDType,
	}, nil
}
