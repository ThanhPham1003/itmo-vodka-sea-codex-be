package services

import (
	"context"
	"sea-codex-server/internal/audit/dto"
	"sea-codex-server/internal/audit/models"
	"sea-codex-server/internal/common"
	"sync"
)

type IAuditService interface {
	common.AuditWriter
	List(context.Context, dto.Filter) (common.List[models.AuditLog], error)
}

var registry struct {
	sync.RWMutex
	service IAuditService
}

func RegisterAuditService(s IAuditService) {
	registry.Lock()
	defer registry.Unlock()
	registry.service = s
}
func AuditService() IAuditService {
	registry.RLock()
	defer registry.RUnlock()
	return registry.service
}
