package dto

import (
	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"time"
)

type Filter struct {
	DossierID *uuid.UUID
	common.Page
	Action     string
	ResourceID *uuid.UUID
	ActorID    *uuid.UUID
	From       *time.Time
	To         *time.Time
}
