package admin

import (
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ListPendingSlashesInput struct {
	Status   string `query:"status"`
	Network  string `query:"network"`
	Page     int    `query:"page" default:"1"`
	PageSize int    `query:"page_size" default:"30"`
}

type GetPendingSlashInput struct {
	PendingSlashID uint `path:"pending_slash_id" validate:"required"`
}

type DownloadPendingSlashArtifactInput struct {
	PendingSlashID   uint   `path:"pending_slash_id" validate:"required"`
	ArtifactType     string `path:"artifact_type" validate:"required"`
	TaskIDCommitment string `path:"task_id_commitment" validate:"required"`
	FileName         string `path:"file_name" validate:"required"`
}

type PendingSlashRecord struct {
	ID               uint                      `json:"id"`
	Status           models.PendingSlashStatus `json:"status"`
	NodeAddress      string                    `json:"node_address"`
	Network          string                    `json:"network"`
	TaskIDCommitment string                    `json:"task_id_commitment"`
	Evidence         *models.SlashEvidence     `json:"evidence"`
	EvidenceComplete bool                      `json:"evidence_complete"`
	CreatedAt        int64                     `json:"created_at"`
	UpdatedAt        int64                     `json:"updated_at"`
}

type ListPendingSlashesData struct {
	Total  int64                `json:"total"`
	Events []PendingSlashRecord `json:"events"`
}

type ListPendingSlashesResponse struct {
	response.Response
	Data ListPendingSlashesData `json:"data"`
}

type GetPendingSlashResponse struct {
	response.Response
	Data PendingSlashRecord `json:"data"`
}

func ListPendingSlashes(c *gin.Context, in *ListPendingSlashesInput) (*ListPendingSlashesResponse, error) {
	page, pageSize := clampSlashReportPagination(in.Page, in.PageSize)
	records, total, err := queryPendingSlashRecords(c.Request.Context(), config.GetDB(), in.Status, in.Network, page, pageSize)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListPendingSlashesResponse{
		Data: ListPendingSlashesData{
			Total:  total,
			Events: records,
		},
	}, nil
}

func GetPendingSlash(c *gin.Context, in *GetPendingSlashInput) (*GetPendingSlashResponse, error) {
	pendingSlash, err := models.GetPendingSlashByID(c.Request.Context(), config.GetDB(), in.PendingSlashID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, response.NewNotFoundErrorResponse()
		}
		return nil, response.NewExceptionResponse(err)
	}
	record, err := buildPendingSlashRecord(pendingSlash)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &GetPendingSlashResponse{Data: *record}, nil
}

func DownloadPendingSlashArtifact(c *gin.Context, in *DownloadPendingSlashArtifactInput) error {
	pendingSlash, err := models.GetPendingSlashByID(c.Request.Context(), config.GetDB(), in.PendingSlashID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.NewNotFoundErrorResponse()
		}
		return response.NewExceptionResponse(err)
	}
	evidence, err := service.ParsePendingSlashEvidence(pendingSlash)
	if err != nil {
		return response.NewExceptionResponse(err)
	}
	artifact, err := findPendingSlashArtifact(evidence, in.ArtifactType, in.TaskIDCommitment)
	if err != nil {
		return err
	}
	fileName, err := cleanArtifactFileName(in.FileName)
	if err != nil {
		return err
	}
	if !artifactContainsFile(artifact, fileName) {
		return response.NewNotFoundErrorResponse()
	}
	filePath := filepath.Join(artifact.StoredPath, fileName)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return response.NewNotFoundErrorResponse()
		}
		return response.NewExceptionResponse(err)
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(fileName))
	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
	return nil
}

func queryPendingSlashRecords(ctx context.Context, db *gorm.DB, status, network string, page, pageSize int) ([]PendingSlashRecord, int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	query := db.WithContext(dbCtx).Model(&models.PendingSlash{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if network != "" {
		query = query.Where("network = ?", network)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pendingSlashes []models.PendingSlash
	if err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&pendingSlashes).Error; err != nil {
		return nil, 0, err
	}
	records := make([]PendingSlashRecord, 0, len(pendingSlashes))
	for i := range pendingSlashes {
		record, err := buildPendingSlashRecord(&pendingSlashes[i])
		if err != nil {
			return nil, 0, err
		}
		records = append(records, *record)
	}
	return records, total, nil
}

func buildPendingSlashRecord(pendingSlash *models.PendingSlash) (*PendingSlashRecord, error) {
	evidence, err := service.ParsePendingSlashEvidence(pendingSlash)
	if err != nil {
		return nil, err
	}
	return &PendingSlashRecord{
		ID:               pendingSlash.ID,
		Status:           pendingSlash.Status,
		NodeAddress:      pendingSlash.NodeAddress,
		Network:          pendingSlash.Network,
		TaskIDCommitment: pendingSlash.TaskIDCommitment,
		Evidence:         evidence,
		EvidenceComplete: pendingSlash.EvidenceComplete,
		CreatedAt:        pendingSlash.CreatedAt.Unix(),
		UpdatedAt:        pendingSlash.UpdatedAt.Unix(),
	}, nil
}

func findPendingSlashArtifact(evidence *models.SlashEvidence, artifactType, taskIDCommitment string) (*models.SlashEvidenceArtifacts, error) {
	var artifacts []models.SlashEvidenceArtifacts
	switch artifactType {
	case "input":
		artifacts = evidence.InputArtifacts
	case "result":
		artifacts = evidence.ResultArtifacts
	default:
		return nil, response.NewValidationErrorResponse("artifact_type", "Invalid artifact type")
	}
	for i := range artifacts {
		if artifacts[i].TaskIDCommitment == taskIDCommitment {
			return &artifacts[i], nil
		}
	}
	return nil, response.NewNotFoundErrorResponse()
}

func cleanArtifactFileName(fileName string) (string, error) {
	cleaned := filepath.Clean(fileName)
	if cleaned == "." || cleaned != filepath.Base(cleaned) {
		return "", response.NewValidationErrorResponse("file_name", "Invalid file name")
	}
	return cleaned, nil
}

func artifactContainsFile(artifact *models.SlashEvidenceArtifacts, fileName string) bool {
	for _, artifactFile := range artifact.Files {
		if filepath.Clean(artifactFile) == fileName {
			return true
		}
	}
	return false
}
