package common

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditEvent metadata must contain references or redacted facts, never source PII.
type AuditEvent struct {
	CompanyID  uuid.UUID
	ActorID    uuid.UUID
	Action     string
	Resource   string
	ResourceID uuid.UUID
	DossierID  *uuid.UUID
	Metadata   map[string]any
}

type AuditWriter interface {
	Write(ctx context.Context, tx *gorm.DB, event AuditEvent) error
}

type BackgroundWorker interface {
	Run(ctx context.Context) error
}
