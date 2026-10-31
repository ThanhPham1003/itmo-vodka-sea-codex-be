package token_test

import (
	"context"
	"strings"
	"testing"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/pkg/token"
)

func TestTokenRoundTrip(t *testing.T) {
	key := paseto.NewV4AsymmetricSecretKey()
	m, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	userID := uuid.New()
	encoded, issued, err := m.CreateToken(context.Background(), userID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(encoded, "v4.public."))
	verified, err := m.VerifyToken(context.Background(), encoded)
	require.NoError(t, err)
	assert.Equal(t, issued, verified)
	assert.Equal(t, userID, verified.UserID)
	assert.WithinDuration(t, time.Now().Add(time.Hour), verified.ExpiresAt, time.Second)
	_, _, err = m.CreateToken(context.Background(), uuid.Nil)
	assert.ErrorIs(t, err, token.ErrInvalidToken)
}

func TestInvalidTokenConfiguration(t *testing.T) {
	key := paseto.NewV4AsymmetricSecretKey()
	other := paseto.NewV4AsymmetricSecretKey()
	for _, tc := range []struct {
		name, private, public string
		lifetime              time.Duration
		want                  error
	}{
		{"missing private", "", key.Public().ExportHex(), time.Hour, token.ErrInvalidKey},
		{"bad private", "do-not-echo-this-private-key", key.Public().ExportHex(), time.Hour, token.ErrInvalidKey},
		{"missing public", key.ExportHex(), "", time.Hour, token.ErrInvalidKey},
		{"mismatch", key.ExportHex(), other.Public().ExportHex(), time.Hour, token.ErrInvalidKey},
		{"zero lifetime", key.ExportHex(), key.Public().ExportHex(), 0, token.ErrInvalidLifetime},
		{"short lifetime", key.ExportHex(), key.Public().ExportHex(), time.Millisecond, token.ErrInvalidLifetime},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := token.NewManager(tc.private, tc.public, tc.lifetime)
			require.ErrorIs(t, err, tc.want)
			assert.NotContains(t, err.Error(), "do-not-echo-this-private-key")
		})
	}
}

func TestRejectInvalidTokens(t *testing.T) {
	key := paseto.NewV4AsymmetricSecretKey()
	other := paseto.NewV4AsymmetricSecretKey()
	m, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	makeToken := func(change func(*paseto.Token), signer paseto.V4AsymmetricSecretKey) string {
		tok := paseto.NewToken()
		now := time.Now().UTC()
		tok.SetJti(uuid.NewString())
		tok.SetSubject(uuid.NewString())
		tok.SetIssuedAt(now.Add(-time.Minute))
		tok.SetNotBefore(now.Add(-time.Minute))
		tok.SetExpiration(now.Add(time.Hour))
		tok.SetIssuer("sea-codex-server")
		tok.SetAudience("sea-codex-api")
		if change != nil {
			change(&tok)
		}
		return tok.V4Sign(signer, nil)
	}
	valid := makeToken(nil, key)
	index := len(valid) / 2
	replacement := "A"
	if valid[index] == 'A' {
		replacement = "B"
	}
	tampered := valid[:index] + replacement + valid[index+1:]
	for _, tc := range []struct{ name, encoded string }{
		{"empty", ""}, {"malformed", "not-a-token"}, {"tampered", tampered},
		{"wrong signing key", makeToken(nil, other)},
		{"expired", makeToken(func(tok *paseto.Token) { tok.SetExpiration(time.Now().Add(-time.Second)) }, key)},
		{"future activation", makeToken(func(tok *paseto.Token) { tok.SetNotBefore(time.Now().Add(time.Minute)) }, key)},
		{"future issuance", makeToken(func(tok *paseto.Token) { tok.SetIssuedAt(time.Now().Add(time.Minute)) }, key)},
		{"wrong audience", makeToken(func(tok *paseto.Token) { tok.SetAudience("other-app") }, key)},
		{"wrong issuer", makeToken(func(tok *paseto.Token) { tok.SetIssuer("other-issuer") }, key)},
		{"invalid subject", makeToken(func(tok *paseto.Token) { tok.SetSubject("not-a-uuid") }, key)},
		{"zero subject", makeToken(func(tok *paseto.Token) { tok.SetSubject(uuid.Nil.String()) }, key)},
		{"missing JTI", makeToken(func(tok *paseto.Token) { tok.SetJti("") }, key)},
		{"oversized", strings.Repeat("a", 8193)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := m.VerifyToken(context.Background(), tc.encoded)
			assert.ErrorIs(t, err, token.ErrInvalidToken)
		})
	}
}

func TestTokenContextCancellation(t *testing.T) {
	key := paseto.NewV4AsymmetricSecretKey()
	m, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = m.CreateToken(ctx, uuid.New())
	assert.ErrorIs(t, err, context.Canceled)
	_, err = m.VerifyToken(ctx, "anything")
	assert.ErrorIs(t, err, context.Canceled)
}
