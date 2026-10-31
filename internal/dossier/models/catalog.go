package models

import (
	"github.com/google/uuid"
	base "sea-codex-server/internal/common/models"
)

type FieldSchema struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	PII      bool   `json:"pii"`
	Pattern  string `json:"pattern,omitempty"`
}
type SpecialRequirement struct {
	Code           string `json:"code"`
	Description    string `json:"description"`
	Severity       string `json:"severity"`
	ExternalSource string `json:"external_source,omitempty"`
}
type DocumentDefinition struct {
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	Fields              []FieldSchema        `json:"fields"`
	SpecialRequirements []SpecialRequirement `json:"special_requirements"`
}
type DocumentType struct {
	base.BaseModel
	CompanyID      uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:idx_document_company_code,priority:1" json:"company_id"`
	Code           string             `gorm:"not null;size:80;uniqueIndex:idx_document_company_code,priority:2" json:"code"`
	CurrentVersion int                `gorm:"not null" json:"current_version"`
	Active         bool               `gorm:"not null" json:"active"`
	Definition     DocumentDefinition `gorm:"serializer:json;type:jsonb" json:"definition"`
}

func (DocumentType) TableName() string { return "company_document_types" }

type DocumentTypeVersion struct {
	base.BaseModel
	CompanyID      uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:idx_document_version,priority:1" json:"company_id"`
	DocumentTypeID uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:idx_document_version,priority:2" json:"document_type_id"`
	Version        int                `gorm:"not null;uniqueIndex:idx_document_version,priority:3" json:"version"`
	Definition     DocumentDefinition `gorm:"serializer:json;type:jsonb" json:"definition"`
}

func (DocumentTypeVersion) TableName() string { return "company_document_type_versions" }

type Condition struct {
	DocumentType string `json:"document_type"`
	FieldKey     string `json:"field_key"`
	Role         string `json:"role,omitempty"`
	Operator     string `json:"operator"`
	Value        string `json:"value,omitempty"`
}
type DocumentRequirement struct {
	Position      int        `json:"position"`
	Key           string     `json:"key"`
	DocumentType  string     `json:"document_type"`
	SchemaVersion int        `json:"schema_version"`
	Role          string     `json:"role,omitempty"`
	Required      bool       `json:"required"`
	Conditional   bool       `json:"conditional"`
	MinCount      int        `json:"min_count"`
	TriggerReason string     `json:"trigger_reason,omitempty"`
	Condition     *Condition `json:"condition,omitempty"`
}
type FieldReference struct {
	DocumentType string `json:"document_type"`
	FieldKey     string `json:"field_key"`
	Role         string `json:"role,omitempty"`
	SubjectField string `json:"subject_field,omitempty"`
}
type ComparisonRule struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Left        FieldReference `json:"left"`
	Right       FieldReference `json:"right"`
	Operator    string         `json:"operator"`
	Severity    string         `json:"severity"`
	Message     string         `json:"message"`
	Remediation string         `json:"remediation"`
}
type DossierDefinition struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Documents   []DocumentRequirement `json:"documents"`
	Rules       []ComparisonRule      `json:"rules"`
}
type DossierType struct {
	base.BaseModel
	CompanyID      uuid.UUID         `gorm:"type:uuid;not null;uniqueIndex:idx_dossier_company_code,priority:1" json:"company_id"`
	Code           string            `gorm:"not null;size:80;uniqueIndex:idx_dossier_company_code,priority:2" json:"code"`
	CurrentVersion int               `gorm:"not null" json:"current_version"`
	Active         bool              `gorm:"not null" json:"active"`
	Definition     DossierDefinition `gorm:"serializer:json;type:jsonb" json:"definition"`
}

func (DossierType) TableName() string { return "company_dossier_types" }

type DossierTypeVersion struct {
	base.BaseModel
	CompanyID     uuid.UUID         `gorm:"type:uuid;not null;uniqueIndex:idx_dossier_type_version,priority:1" json:"company_id"`
	DossierTypeID uuid.UUID         `gorm:"type:uuid;not null;uniqueIndex:idx_dossier_type_version,priority:2" json:"dossier_type_id"`
	Version       int               `gorm:"not null;uniqueIndex:idx_dossier_type_version,priority:3" json:"version"`
	Definition    DossierDefinition `gorm:"serializer:json;type:jsonb" json:"definition"`
}

func (DossierTypeVersion) TableName() string { return "company_dossier_type_versions" }

type PinnedDocument struct {
	Code       string             `json:"code"`
	Version    int                `json:"version"`
	Definition DocumentDefinition `json:"definition"`
}
type DossierSnapshot struct {
	Code       string            `json:"code"`
	Version    int               `json:"version"`
	Definition DossierDefinition `json:"definition"`
	Documents  []PinnedDocument  `json:"documents"`
}
