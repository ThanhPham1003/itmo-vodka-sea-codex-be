package services

import (
	"context"
	"github.com/google/uuid"
	"os"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sync"
)

type Content struct {
	File *os.File
	Name string
	MIME string
	Size int64
}
type IDossierService interface {
	Create(context.Context, dto.CreateDossierRequest) (models.Dossier, error)
	List(context.Context, dto.DossierFilter, bool) (common.List[models.Dossier], error)
	Detail(context.Context, uuid.UUID) (dto.Detail, error)
	Upload(context.Context, uuid.UUID, []dto.UploadInput) (dto.UploadResult, error)
	Retry(context.Context, uuid.UUID, uuid.UUID, uint64) error
	Classify(context.Context, uuid.UUID, uuid.UUID, dto.ClassificationRequest) error
	Assign(context.Context, uuid.UUID, dto.AssignmentRequest) error
	AddRequirement(context.Context, uuid.UUID, dto.RequirementRequest) error
	WaiveRequirement(context.Context, uuid.UUID, uuid.UUID, dto.ReasonRequest) error
	Correct(context.Context, uuid.UUID, uuid.UUID, dto.CorrectionRequest) error
	Acknowledge(context.Context, uuid.UUID, uuid.UUID, dto.ReasonRequest) error
	Review(context.Context, uuid.UUID, dto.ReviewRequest) (models.Decision, error)
	Reopen(context.Context, uuid.UUID, dto.ReasonRequest) error
	Reveal(context.Context, uuid.UUID, dto.RevealRequest) ([]models.ExtractedField, error)
	FileContent(context.Context, uuid.UUID, uuid.UUID) (Content, error)
	Dashboard(context.Context, string, string, string, string, string) (any, error)
}

var dossierRegistry struct {
	sync.RWMutex
	service IDossierService
}

func RegisterDossierService(s IDossierService) {
	dossierRegistry.Lock()
	defer dossierRegistry.Unlock()
	dossierRegistry.service = s
}
func DossierService() IDossierService {
	dossierRegistry.RLock()
	defer dossierRegistry.RUnlock()
	return dossierRegistry.service
}
