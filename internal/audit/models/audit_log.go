package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLog struct {
	DossierID *uuid.UUID `gorm:"type:uuid;index" json:"dossier_id,omitempty"`
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	CompanyID uuid.UUID  `gorm:"type:uuid;not null;index:idx_audit_company_time,priority:1" json:"company_id"`
	ActorID   uuid.UUID  `gorm:"type:uuid;not null" json:"actor_id"`
	// Resolved on reads only; existing immutable events and database schema stay intact.
	ActorName    string         `gorm:"->;-:migration" json:"actor_name"`
	ResourceName string         `gorm:"-" json:"resource_name"`
	DossierTitle string         `gorm:"-" json:"dossier_title"`
	Action       string         `gorm:"not null;size:100;index" json:"action"`
	Resource     string         `gorm:"not null;size:100" json:"resource"`
	ResourceID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"resource_id"`
	Metadata     map[string]any `gorm:"serializer:json;type:jsonb" json:"metadata"`
	CreatedAt    time.Time      `gorm:"not null;index:idx_audit_company_time,priority:2" json:"created_at"`
}

func (a *AuditLog) BeforeCreate(_ *gorm.DB) error {
	if a.CompanyID == uuid.Nil || a.Action == "" {
		return errors.New("audit company and action are required")
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
func (*AuditLog) BeforeUpdate(*gorm.DB) error { return errors.New("audit events are immutable") }
func (*AuditLog) BeforeDelete(*gorm.DB) error { return errors.New("audit events are immutable") }
