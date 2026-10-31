// Package token issues and verifies signed PASETO v4.public identity tokens.
package token

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrInvalidKey      = errors.New("invalid PASETO key pair")
	ErrInvalidLifetime = errors.New("token lifetime must be at least one second")
)

// TokenClaims deliberately contains no tenant, role or permissions.
// Authorization must resolve the current user and company from trusted storage.
type TokenClaims struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	IssuedAt  time.Time `json:"issued_at"`
	NotBefore time.Time `json:"not_before"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ITokenManager interface {
	CreateToken(ctx context.Context, userID uuid.UUID) (string, TokenClaims, error)
	VerifyToken(ctx context.Context, encoded string) (TokenClaims, error)
}
