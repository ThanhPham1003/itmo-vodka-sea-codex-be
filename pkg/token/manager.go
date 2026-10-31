package token

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/google/uuid"
)

const (
	tokenIssuer   = "sea-codex-server"
	tokenAudience = "sea-codex-api"
)

type Manager struct {
	privateKey paseto.V4AsymmetricSecretKey
	publicKey  paseto.V4AsymmetricPublicKey
	lifetime   time.Duration
}

var _ ITokenManager = (*Manager)(nil)

func NewManager(privateKeyHex, publicKeyHex string, lifetime time.Duration) (*Manager, error) {
	if lifetime < time.Second {
		return nil, ErrInvalidLifetime
	}
	privateKey, err := paseto.NewV4AsymmetricSecretKeyFromHex(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("%w: private key", ErrInvalidKey)
	}
	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromHex(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("%w: public key", ErrInvalidKey)
	}
	if subtle.ConstantTimeCompare(privateKey.Public().ExportBytes(), publicKey.ExportBytes()) != 1 {
		return nil, fmt.Errorf("%w: keys do not match", ErrInvalidKey)
	}
	return &Manager{privateKey: privateKey, publicKey: publicKey, lifetime: lifetime}, nil
}

func (m *Manager) CreateToken(ctx context.Context, userID uuid.UUID) (string, TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return "", TokenClaims{}, fmt.Errorf("create token: %w", err)
	}
	if userID == uuid.Nil {
		return "", TokenClaims{}, fmt.Errorf("%w: user ID is required", ErrInvalidToken)
	}
	now := time.Now().UTC().Truncate(time.Second)
	claims := TokenClaims{
		ID: uuid.New(), UserID: userID, IssuedAt: now, NotBefore: now,
		ExpiresAt: now.Add(m.lifetime).Truncate(time.Second),
	}
	t := paseto.NewToken()
	t.SetJti(claims.ID.String())
	t.SetSubject(userID.String())
	t.SetIssuedAt(claims.IssuedAt)
	t.SetNotBefore(claims.NotBefore)
	t.SetExpiration(claims.ExpiresAt)
	t.SetIssuer(tokenIssuer)
	t.SetAudience(tokenAudience)
	return t.V4Sign(m.privateKey, nil), claims, nil
}

func (m *Manager) VerifyToken(ctx context.Context, encoded string) (TokenClaims, error) {
	if err := ctx.Err(); err != nil {
		return TokenClaims{}, fmt.Errorf("verify token: %w", err)
	}
	if len(encoded) > 8192 {
		return TokenClaims{}, ErrInvalidToken
	}
	parser := paseto.MakeParser([]paseto.Rule{
		paseto.ValidAt(time.Now().UTC()), paseto.IssuedBy(tokenIssuer), paseto.ForAudience(tokenAudience),
	})
	t, err := parser.ParseV4Public(m.publicKey, encoded, nil)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	subject, err := t.GetSubject()
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(subject)
	if err != nil || userID == uuid.Nil {
		return TokenClaims{}, ErrInvalidToken
	}
	idString, err := t.GetJti()
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	id, err := uuid.Parse(idString)
	if err != nil || id == uuid.Nil {
		return TokenClaims{}, ErrInvalidToken
	}
	iat, e1 := t.GetIssuedAt()
	nbf, e2 := t.GetNotBefore()
	exp, e3 := t.GetExpiration()
	if e1 != nil || e2 != nil || e3 != nil || !exp.After(iat) || !exp.After(nbf) {
		return TokenClaims{}, ErrInvalidToken
	}
	return TokenClaims{ID: id, UserID: userID, IssuedAt: iat, NotBefore: nbf, ExpiresAt: exp}, nil
}
