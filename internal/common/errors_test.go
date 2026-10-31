package common_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"sea-codex-server/internal/common"
)

func TestHTTPErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"input", common.ErrInvalidArgument, 400},
		{"authentication", common.ErrUnauthenticated, 401},
		{"permission", common.ErrForbidden, 403},
		{"missing or other tenant", common.ErrNotFound, 404},
		{"conflict", common.ErrConflict, 409},
		{"dependency", common.ErrUnavailable, 503},
		{"unknown", errors.New("private-secret-cause"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, message := common.HTTPError(fmt.Errorf("private-secret-cause: %w", tc.err))
			assert.Equal(t, tc.status, status)
			assert.NotContains(t, message, "private-secret-cause")
		})
	}
}
