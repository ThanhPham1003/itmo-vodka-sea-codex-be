package utils_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/pkg/utils"
)

func TestPasswordHash(t *testing.T) {
	hash, err := utils.HashPassword("test-password")
	require.NoError(t, err)
	assert.NotEqual(t, "test-password", hash)
	for _, tc := range []struct {
		name, password, hash string
		want                 bool
	}{
		{"matching", "test-password", hash, true},
		{"wrong", "other-password", hash, false},
		{"invalid hash", "test-password", "not-a-hash", false},
		{"empty", "", hash, false},
		{"oversized", strings.Repeat("a", 73), hash, false},
	} {
		t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.want, utils.CheckPasswordHash(tc.password, tc.hash)) })
	}
	for _, password := range []string{"", strings.Repeat("a", 73), strings.Repeat("à", 37)} {
		_, err := utils.HashPassword(password)
		assert.Error(t, err)
	}
}
