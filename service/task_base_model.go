package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service/huggingface"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	ErrInvalidBaseModel          = errors.New("invalid base model")
	ErrBaseModelCheckUnavailable = errors.New("base model check temporarily unavailable")
)

var (
	hfClientMu sync.Mutex
	hfClient   *huggingface.Client
)

func getHuggingFaceClient() *huggingface.Client {
	hfClientMu.Lock()
	defer hfClientMu.Unlock()
	if hfClient != nil {
		return hfClient
	}
	cfg := config.GetConfig().HuggingFace
	hfClient = huggingface.NewClient(
		cfg.APIBaseURL,
		time.Duration(cfg.TimeoutSeconds*float64(time.Second)),
		time.Duration(cfg.CacheTTLSeconds*float64(time.Second)),
	)
	return hfClient
}

// SetHuggingFaceClientForTest replaces the Hub client used by ValidateTaskBaseModel.
func SetHuggingFaceClientForTest(client *huggingface.Client) {
	hfClientMu.Lock()
	defer hfClientMu.Unlock()
	hfClient = client
}

// ValidateTaskBaseModel checks that each huggingface base model is either already
// present on a node (exact dispatch ID in node_models) or exists on the Hub.
// URL-based base models skip Hub validation.
func ValidateTaskBaseModel(ctx context.Context, db *gorm.DB, taskType models.TaskType, modelIDs []string) error {
	baseIDs := models.BaseModelIDs(modelIDs)
	if len(baseIDs) == 0 {
		return nil
	}
	kind := huggingface.ModelKindSD
	if taskType == models.TaskTypeLLM {
		kind = huggingface.ModelKindLLM
	}
	client := getHuggingFaceClient()
	for _, baseID := range baseIDs {
		hfModelID, ok := models.BaseModelHuggingFaceID(baseID)
		if !ok {
			continue
		}
		exists, err := models.NodeModelExists(ctx, db, baseID)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		variant, _ := models.BaseModelVariant(baseID)
		if err := client.ValidateBaseModel(ctx, kind, hfModelID, variant); err != nil {
			if errors.Is(err, huggingface.ErrServiceUnavailable) {
				return ErrBaseModelCheckUnavailable
			}
			if errors.Is(err, huggingface.ErrVariantUnavailable) {
				return errors.Join(ErrInvalidBaseModel, err)
			}
			if errors.Is(err, huggingface.ErrModelNotFound) {
				return errors.Join(ErrInvalidBaseModel, err)
			}
			if strings.TrimSpace(err.Error()) == "" {
				return ErrInvalidBaseModel
			}
			return errors.Join(ErrInvalidBaseModel, err)
		}
	}
	return nil
}
