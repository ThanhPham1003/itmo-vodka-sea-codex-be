package common

import (
	"context"

	"github.com/google/uuid"
)

// IdentityResolver must load the active user, active company and current permissions.
// The auth module will provide the database implementation. Return
// ErrUnauthenticated for deleted/inactive users or companies, and wrap infrastructure
// errors separately so the middleware can distinguish 401 from 503.
type IdentityResolver interface {
	ResolveIdentity(ctx context.Context, userID uuid.UUID) (Actor, error)
}
