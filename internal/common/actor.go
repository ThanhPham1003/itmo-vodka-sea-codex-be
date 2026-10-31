package common

import (
	"context"

	"github.com/google/uuid"
)

const (
	RoleCompanyAdmin        = "company_admin"
	PermissionPIIView       = "pii:view"
	PermissionDashboardRead = "dashboard:read"
)

type Actor struct {
	UserID      uuid.UUID `json:"user_id"`
	CompanyID   uuid.UUID `json:"company_id"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Permissions []string  `json:"permissions"`
}

func (a Actor) HasPermission(permission string) bool {
	// Only these two defaults are guaranteed by the current product design.
	// Company admin is never a global authorization or tenant-scope bypass.
	if a.Role == RoleCompanyAdmin && (permission == PermissionPIIView || permission == PermissionDashboardRead) {
		return true
	}
	for _, granted := range a.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}

type actorContextKey struct{}
type requestIDContextKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	actor.Permissions = append([]string(nil), actor.Permissions...)
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	if !ok || actor.UserID == uuid.Nil || actor.CompanyID == uuid.Nil {
		return Actor{}, false
	}
	actor.Permissions = append([]string(nil), actor.Permissions...)
	return actor, true
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}
