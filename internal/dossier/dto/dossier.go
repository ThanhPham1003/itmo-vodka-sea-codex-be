package dto

import (
	"github.com/google/uuid"
	"io"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/models"
	"time"
)

type CreateDossierRequest struct {
	Title       string `json:"title" binding:"required,max=300"`
	Priority    int    `json:"priority" binding:"gte=0,lte=5"`
	DossierType string `json:"dossier_type" binding:"required,max=80"`
}
type VersionRequest struct {
	ExpectedVersion uint64 `json:"expected_version" binding:"required"`
}
type ReasonRequest struct {
	ExpectedVersion uint64 `json:"expected_version" binding:"required"`
	Reason          string `json:"reason" binding:"required,max=2000"`
}
type AssignmentRequest struct {
	Priority        *int       `json:"priority" binding:"omitempty,gte=0,lte=5"`
	ExpectedVersion uint64     `json:"expected_version" binding:"required"`
	AssigneeID      *uuid.UUID `json:"assignee_id"`
}
type RequirementRequest struct {
	ExpectedVersion uint64 `json:"expected_version" binding:"required"`
	DocumentType    string `json:"document_type" binding:"required"`
	Role            string `json:"role" binding:"max=80"`
	Reason          string `json:"reason" binding:"required,max=2000"`
	MinCount        int    `json:"min_count"`
}
type CorrectionRequest struct {
	ExpectedVersion uint64 `json:"expected_version" binding:"required"`
	Value           string `json:"value" binding:"max=20000"`
	Reason          string `json:"reason" binding:"required"`
	Note            string `json:"note" binding:"max=2000"`
}
type ClassificationRequest struct {
	ExpectedVersion uint64     `json:"expected_version" binding:"required"`
	DocumentType    string     `json:"document_type" binding:"required"`
	Role            string     `json:"role" binding:"max=80"`
	GroupID         *uuid.UUID `json:"document_group_id"`
}
type ReviewRequest struct {
	ExpectedVersion uint64 `json:"expected_version" binding:"required"`
	Action          string `json:"action" binding:"required,oneof=APPROVE REJECT REQUEST_CHANGES"`
	Reason          string `json:"reason" binding:"max=2000"`
}
type RevealRequest struct {
	FieldIDs []uuid.UUID `json:"field_ids" binding:"required,min=1,max=100"`
}
type UploadInput struct {
	Name           string
	DocumentType   string
	Role           string
	Open           func() (io.ReadCloser, error)
	ReplacesFileID *uuid.UUID
}
type UploadedFile struct {
	ID     *uuid.UUID `json:"id,omitempty"`
	Name   string     `json:"name"`
	Status string     `json:"status"`
	Error  string     `json:"error,omitempty"`
}
type UploadResult struct {
	BatchID uuid.UUID      `json:"batch_id"`
	Files   []UploadedFile `json:"files"`
}
type Detail struct {
	Dossier      models.Dossier          `json:"dossier"`
	Requirements []models.Requirement    `json:"checklist"`
	Files        []models.File           `json:"files"`
	Groups       []models.Group          `json:"document_groups"`
	Fields       []models.ExtractedField `json:"fields"`
	Issues       []models.Issue          `json:"issues"`
	Decisions    []models.Decision       `json:"decisions"`
}
type DossierFilter struct {
	Review bool
	common.Page
	Status      string
	Type        string
	Search      string
	CreatorID   *uuid.UUID
	AssigneeID  *uuid.UUID
	HistoryFrom *time.Time
	HistoryTo   *time.Time
}
