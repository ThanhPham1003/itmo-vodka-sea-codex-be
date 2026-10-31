package models

import (
	"github.com/google/uuid"
	base "sea-codex-server/internal/common/models"
)

type Evidence struct {
	FileID   uuid.UUID `json:"file_id"`
	GroupID  uuid.UUID `json:"document_group_id"`
	FieldID  uuid.UUID `json:"field_id"`
	FieldKey string    `json:"field_key"`
	Page     int       `json:"page"`
	Value    string    `json:"value"`
}
type Issue struct {
	base.BaseModel
	CompanyID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"dossier_id"`
	RuleCode        string     `gorm:"size:100;not null" json:"rule_code"`
	Code            string     `gorm:"size:100;not null" json:"code"`
	Severity        string     `gorm:"size:20;not null" json:"severity"`
	Status          string     `gorm:"size:30;not null" json:"status"`
	Message         string     `json:"message"`
	Remediation     string     `json:"remediation"`
	Evidence        []Evidence `gorm:"serializer:json;type:jsonb" json:"evidence"`
	Fingerprint     string     `gorm:"not null" json:"-"`
	AcknowledgedBy  *uuid.UUID `gorm:"type:uuid" json:"acknowledged_by"`
	Acknowledgement string     `json:"acknowledgement,omitempty"`
}

func (Issue) TableName() string { return "dossier_reconciliation_issues" }

type Decision struct {
	base.BaseModel
	CompanyID uuid.UUID `gorm:"type:uuid;not null;index" json:"company_id"`
	DossierID uuid.UUID `gorm:"type:uuid;not null;index" json:"dossier_id"`
	ActorID   uuid.UUID `gorm:"type:uuid;not null" json:"actor_id"`
	ActorName string    `gorm:"-" json:"actor_name"`
	Action    string    `gorm:"size:40;not null" json:"action"`
	Reason    string    `json:"reason"`
	Version   uint64    `gorm:"not null" json:"version"`
	Snapshot  string    `gorm:"type:text;not null" json:"-"`
}

func (Decision) TableName() string { return "dossier_decisions" }
