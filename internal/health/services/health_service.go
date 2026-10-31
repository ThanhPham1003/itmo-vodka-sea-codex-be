// Package services defines the health contract and startup-only service registry.
package services

import (
	"context"
	"sync"

	"sea-codex-server/internal/health/dto"
)

type IHealthService interface {
	Check(ctx context.Context) (dto.HealthResponse, error)
}

var registry struct {
	sync.RWMutex
	health IHealthService
}

// RegisterHealthService installs the service once, before accepting requests.
// It returns false for nil or repeated registrations instead of overwriting state.
func RegisterHealthService(service IHealthService) bool {
	registry.Lock()
	defer registry.Unlock()
	if service == nil || registry.health != nil {
		return false
	}
	registry.health = service
	return true
}

func HealthService() IHealthService {
	registry.RLock()
	defer registry.RUnlock()
	return registry.health
}
