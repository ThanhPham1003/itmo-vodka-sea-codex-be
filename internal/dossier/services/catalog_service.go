package services

import (
	"context"
	"encoding/json"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sync"
)

type ICatalogService interface {
	ListDocumentTypes(context.Context, common.Page) (common.List[models.DocumentType], error)
	GetDocumentType(context.Context, string) (models.DocumentType, error)
	PutDocumentType(context.Context, string, dto.DocumentTypeRequest) (models.DocumentType, error)
	ArchiveDocumentType(context.Context, string) error
	ListDossierTypes(context.Context, common.Page) (common.List[models.DossierType], error)
	GetDossierType(context.Context, string) (models.DossierType, error)
	PutDossierType(context.Context, string, dto.DossierTypeRequest) (models.DossierType, error)
	ArchiveDossierType(context.Context, string) error
	SuggestDocument(context.Context, string) (json.RawMessage, error)
	SuggestDossier(context.Context, string, []string) (json.RawMessage, error)
	SuggestRules(context.Context, string, string) (json.RawMessage, error)
}

var catalogRegistry struct {
	sync.RWMutex
	service ICatalogService
}

func RegisterCatalogService(s ICatalogService) {
	catalogRegistry.Lock()
	defer catalogRegistry.Unlock()
	catalogRegistry.service = s
}
func CatalogService() ICatalogService {
	catalogRegistry.RLock()
	defer catalogRegistry.RUnlock()
	return catalogRegistry.service
}
