package dto

import "sea-codex-server/internal/dossier/models"

type DocumentTypeRequest struct {
	models.DocumentDefinition
	Active          *bool `json:"active"`
	ExpectedVersion int   `json:"expected_version"`
}
type DossierTypeRequest struct {
	models.DossierDefinition
	Active          *bool `json:"active"`
	ExpectedVersion int   `json:"expected_version"`
}
type SuggestionRequest struct {
	Description string `json:"description" binding:"required,max=4000"`
}

type DraftRuleSuggestionRequest struct {
	Description string                   `json:"description" binding:"required,max=4000"`
	Definition  models.DossierDefinition `json:"definition" binding:"required"`
}
