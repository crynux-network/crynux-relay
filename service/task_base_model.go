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
	ErrInvalidTaskModelArgs      = errors.New("invalid task model args")
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

// PrepareTaskModelRequirements derives the model file requirements of a task
// from its TaskArgs and registers the exact model files in the catalog.
//
// A file that no node holds is checked on the Hub. An entry with a revision
// is resolved to its commit SHA, so the exact hash always names a commit. An
// entry without a revision keeps its revision-less source identity.
func PrepareTaskModelRequirements(ctx context.Context, db *gorm.DB, taskType models.TaskType, taskArgs string) (*models.TaskModelRequirements, error) {
	requirements, err := models.DeriveTaskModelRequirements(taskArgs, taskType)
	if err != nil {
		return nil, errors.Join(ErrInvalidTaskModelArgs, err)
	}

	client := getHuggingFaceClient()
	revisions := make(map[int]string)
	for index, entry := range requirements.Entries {
		if entry.Revision != "" {
			hash, err := entry.ModelFileHash()
			if err != nil {
				return nil, err
			}
			held, err := models.NodeModelExists(ctx, db, hash)
			if err != nil {
				return nil, err
			}
			if held {
				continue
			}
			sha, err := client.ResolveRevision(ctx, entry.Repository, entry.Revision)
			if err != nil {
				return nil, hubValidationError(err)
			}
			revisions[index] = sha
			continue
		}

		sourceHash, err := entry.ModelFileSourceHash()
		if err != nil {
			return nil, err
		}
		held, err := models.NodeModelSourceExists(ctx, db, sourceHash)
		if err != nil {
			return nil, err
		}
		if held {
			continue
		}
		// SD entries[0] is always the base pipeline from DeriveTaskModelRequirements.
		requirePipeline := taskType == models.TaskTypeSD && index == 0
		if err := validateEntryOnHub(ctx, client, entry, requirePipeline); err != nil {
			return nil, hubValidationError(err)
		}
	}

	if len(revisions) > 0 {
		if err := requirements.ResolveRevisions(revisions); err != nil {
			return nil, err
		}
	}
	files, err := requirements.ModelFiles()
	if err != nil {
		return nil, err
	}
	if err := models.UpsertModelFiles(ctx, db, files); err != nil {
		return nil, err
	}
	return requirements, nil
}

// validateEntryOnHub checks that the repository of a model entry exists.
// requirePipeline is true only for the SD base_model entry; other entries,
// including Diffusers-format components, are validated as public repositories.
func validateEntryOnHub(ctx context.Context, client *huggingface.Client, entry models.ModelFileEntry, requirePipeline bool) error {
	if !requirePipeline {
		return client.ValidateBaseModel(ctx, huggingface.ModelKindRepository, entry.Repository, "")
	}
	variant, _ := entry.FormatKwargs["variant"].(string)
	return client.ValidateBaseModel(ctx, huggingface.ModelKindSD, entry.Repository, variant)
}

func hubValidationError(err error) error {
	if errors.Is(err, huggingface.ErrServiceUnavailable) {
		return ErrBaseModelCheckUnavailable
	}
	if strings.TrimSpace(err.Error()) == "" {
		return ErrInvalidBaseModel
	}
	return errors.Join(ErrInvalidBaseModel, err)
}
