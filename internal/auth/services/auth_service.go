package services

import (
	"context"
	"github.com/google/uuid"
	"sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/common"
	"sync"
)

type IAuthService interface {
	common.IdentityResolver
	Signup(context.Context, dto.SignupRequest) (dto.UserResponse, error)
	Login(context.Context, dto.LoginRequest) (dto.LoginResponse, error)
	Me(context.Context) (dto.UserResponse, error)
	ListUsers(context.Context, common.Page) (common.List[dto.UserResponse], error)
	CreateUser(context.Context, dto.CreateUserRequest) (dto.UserResponse, error)
	UpdateAccess(context.Context, uuid.UUID, dto.UpdateAccessRequest) (dto.UserResponse, error)
}

var registry struct {
	sync.RWMutex
	service IAuthService
}

func RegisterAuthService(s IAuthService) {
	registry.Lock()
	defer registry.Unlock()
	registry.service = s
}
func AuthService() IAuthService { registry.RLock(); defer registry.RUnlock(); return registry.service }
