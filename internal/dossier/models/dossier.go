package models

import (
	"github.com/google/uuid"
	base "sea-codex-server/internal/common/models"
	"time"
)

const (
	Processing = "PROCESSING"
	Missing    = "MISSING_DOCUMENT"
	Invalid    = "INVALID_DOCUMENT"
	Valid      = "VALID_DOCUMENT"
	Approved   = "APPROVED_DOCUMENT"
	Rejected   = "REJECTED"
)

type Dossier struct {
	base.BaseModel
	CompanyID     uuid.UUID       `gorm:"type:uuid;not null;index:idx_dossier_scope,priority:1" json:"company_id"`
	CreatorID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"creator_id"`
	AssigneeID    *uuid.UUID      `gorm:"type:uuid;index" json:"assignee_id"`
	Title         string          `gorm:"not null;size:300" json:"title"`
	Priority      int             `gorm:"not null" json:"priority"`
	TypeCode      string          `gorm:"not null;size:80;index" json:"dossier_type"`
	TypeVersion   int             `gorm:"not null" json:"dossier_type_version"`
	Version       uint64          `gorm:"not null" json:"version"`
	Status        string          `gorm:"not null;size:40;index:idx_dossier_scope,priority:2" json:"status"`
	Snapshot      DossierSnapshot `gorm:"serializer:json;type:jsonb" json:"configuration"`
	ChangeRequest string          `json:"change_request,omitempty"`
}
type Requirement struct {
	Position int `json:"position"`
	base.BaseModel
	CompanyID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"dossier_id"`
	Key          string     `gorm:"not null;size:80" json:"key"`
	DocumentType string     `gorm:"not null;size:80" json:"document_type"`
	Role         string     `gorm:"size:80" json:"role"`
	Name         string     `gorm:"not null" json:"name"`
	Required     bool       `json:"required"`
	Conditional  bool       `json:"conditional"`
	MinCount     int        `json:"min_count"`
	Condition    *Condition `gorm:"serializer:json;type:jsonb" json:"condition,omitempty"`
	Override     *bool      `json:"override,omitempty"`
	Reason       string     `json:"reason"`
	Status       string     `gorm:"not null;size:40" json:"status"`
}

func (Requirement) TableName() string { return "dossier_document_requirements" }

type StatusHistory struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	CompanyID uuid.UUID `gorm:"type:uuid;not null;index:idx_status_history_scope,priority:1" json:"company_id"`
	DossierID uuid.UUID `gorm:"type:uuid;not null;index" json:"dossier_id"`
	From      string    `json:"from"`
	To        string    `gorm:"not null;index:idx_status_history_scope,priority:2" json:"to"`
	ActorID   uuid.UUID `gorm:"type:uuid;not null" json:"actor_id"`
	CreatedAt time.Time `gorm:"not null;index:idx_status_history_scope,priority:3" json:"created_at"`
}

func (StatusHistory) TableName() string { return "dossier_status_history" }
