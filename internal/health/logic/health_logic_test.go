package logic_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/health/logic"
)

func TestHealthCheck(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "running", true: "context cancelled"}[cancelled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			result, err := logic.NewHealthLogic().Check(ctx)
			if cancelled {
				assert.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "ok", result.Status)
			}
		})
	}
}
