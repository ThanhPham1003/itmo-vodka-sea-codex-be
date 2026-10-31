// Package logic implements process liveness without consulting external dependencies.
package logic

import (
	"context"
	"fmt"

	"sea-codex-server/internal/health/dto"
	"sea-codex-server/internal/health/services"
)

type HealthLogic struct{}

var _ services.IHealthService = (*HealthLogic)(nil)

func NewHealthLogic() *HealthLogic { return &HealthLogic{} }

func (l *HealthLogic) Check(ctx context.Context) (dto.HealthResponse, error) {
	if err := ctx.Err(); err != nil {
		return dto.HealthResponse{}, fmt.Errorf("check liveness: %w", err)
	}
	return dto.HealthResponse{Status: "ok"}, nil
}
