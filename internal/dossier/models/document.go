package models

import (
	"github.com/google/uuid"
	base "sea-codex-server/internal/common/models"
	"time"
)

type File struct {
	base.BaseModel
	CompanyID            uuid.UUID                `gorm:"type:uuid;not null;index:idx_file_scope,priority:1" json:"company_id"`
	DossierID            uuid.UUID                `gorm:"type:uuid;not null;index:idx_file_scope,priority:2" json:"dossier_id"`
	BatchID              uuid.UUID                `gorm:"type:uuid;not null" json:"batch_id"`
	SourceFileID         *uuid.UUID               `gorm:"type:uuid;index" json:"source_file_id"`
	SupersededBy         *uuid.UUID               `gorm:"type:uuid" json:"superseded_by,omitempty"`
	OriginalName         string                   `gorm:"not null;size:255" json:"original_name"`
	StorageKey           string                   `gorm:"not null" json:"-"`
	SHA256               string                   `gorm:"not null;size:64;index" json:"sha256"`
	MIME                 string                   `gorm:"not null;size:100" json:"mime"`
	Size                 int64                    `gorm:"not null" json:"size"`
	DocumentType         string                   `gorm:"size:80" json:"document_type"`
	Role                 string                   `gorm:"size:80" json:"role"`
	Status               string                   `gorm:"not null;size:40;index:idx_file_queue,priority:1" json:"status"`
	ErrorCode            string                   `gorm:"size:80" json:"error_code,omitempty"`
	ErrorMessage         string                   `json:"error_message,omitempty"`
	Pages                []int                    `gorm:"serializer:json;type:jsonb" json:"source_pages"`
	Attempt              int                      `gorm:"not null" json:"attempt"`
	ClaimID              *uuid.UUID               `gorm:"type:uuid" json:"-"`
	LeaseUntil           *time.Time               `gorm:"index:idx_file_queue,priority:2" json:"-"`
	ModelVersion         string                   `json:"model_version"`
	SchemaVersion        int                      `json:"schema_version"`
	Overrides            []ClassificationOverride `gorm:"serializer:json;type:jsonb" json:"classification_overrides,omitempty"`
	PreservedCorrections []PreservedCorrection    `gorm:"serializer:json;type:jsonb" json:"-"`
}
type PreservedCorrection struct {
	DocumentType string `json:"document_type"`
	Role         string `json:"role"`
	Pages        []int  `json:"pages"`
	Key          string `json:"key"`
	Value        string `json:"value"`
}
type ClassificationOverride struct {
	Pages        []int  `json:"pages"`
	DocumentType string `json:"document_type"`
	Role         string `json:"role"`
}

func (File) TableName() string { return "dossier_files" }

type Group struct {
	base.BaseModel
	CompanyID         uuid.UUID `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID         uuid.UUID `gorm:"type:uuid;not null;index" json:"dossier_id"`
	FileID            uuid.UUID `gorm:"type:uuid;not null;index" json:"file_id"`
	DocumentType      string    `gorm:"size:80" json:"document_type"`
	Role              string    `gorm:"size:80" json:"role"`
	Pages             []int     `gorm:"serializer:json;type:jsonb" json:"pages"`
	Confidence        float64   `json:"confidence"`
	Confirmed         bool      `json:"confirmed"`
	NeedsConfirmation bool      `json:"needs_confirmation"`
}

func (Group) TableName() string { return "dossier_document_groups" }

type ExtractedField struct {
	base.BaseModel
	CompanyID       uuid.UUID `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID       uuid.UUID `gorm:"type:uuid;not null;index" json:"dossier_id"`
	FileID          uuid.UUID `gorm:"type:uuid;not null;index" json:"file_id"`
	GroupID         uuid.UUID `gorm:"type:uuid;not null;index" json:"document_group_id"`
	Key             string    `gorm:"size:80;not null" json:"key"`
	Label           string    `json:"label"`
	Type            string    `gorm:"size:40" json:"type"`
	Value           string    `json:"value"`
	MachineValue    string    `json:"-"`
	RawText         string    `json:"raw_text"`
	NormalizedValue string    `json:"normalized_value"`
	Confidence      float64   `json:"confidence"`
	PII             bool      `json:"pii"`
	Required        bool      `json:"required"`
	Validation      string    `json:"validation"`
	Page            int       `json:"page"`
	BoundingBox     []float64 `gorm:"serializer:json;type:jsonb" json:"bounding_box"`
	Corrected       bool      `json:"corrected"`
	SchemaVersion   int       `json:"schema_version"`
}

func (ExtractedField) TableName() string { return "dossier_extracted_fields" }

type FieldCorrection struct {
	base.BaseModel
	CompanyID uuid.UUID `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID uuid.UUID `gorm:"type:uuid;not null;index" json:"dossier_id"`
	FieldID   uuid.UUID `gorm:"type:uuid;not null" json:"field_id"`
	ActorID   uuid.UUID `gorm:"type:uuid;not null" json:"actor_id"`
	Before    string    `json:"-"`
	After     string    `json:"-"`
	Reason    string    `gorm:"size:80;not null" json:"reason"`
	Note      string    `json:"-"`
}

func (FieldCorrection) TableName() string { return "dossier_field_corrections" }

type SpecialCheck struct {
	base.BaseModel
	CompanyID  uuid.UUID `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID  uuid.UUID `gorm:"type:uuid;not null;index" json:"dossier_id"`
	FileID     uuid.UUID `gorm:"type:uuid;not null;index" json:"file_id"`
	GroupID    uuid.UUID `gorm:"type:uuid;not null" json:"document_group_id"`
	Code       string    `json:"code"`
	Status     string    `json:"status"`
	Confidence float64   `json:"confidence"`
	Evidence   string    `json:"-"`
	Page       int       `json:"page"`
}

func (SpecialCheck) TableName() string { return "dossier_special_checks" }
