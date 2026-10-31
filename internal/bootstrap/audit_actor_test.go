package bootstrap_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/audit/models"
	"sea-codex-server/internal/bootstrap"
	"sea-codex-server/internal/common"
	"testing"
)

func TestAuditAPIIncludesActorName(t *testing.T) {
	app := newTestApp(t)
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	u, token := signupAndLogin(t, router, "auditnames")
	out := data[common.List[models.AuditLog]](t, request(t, router, "GET", "/api/v1/audit-logs?action=LOGIN&actor_id="+u.ID.String(), token, nil), 200)
	require.NotEmpty(t, out.Items)
	for _, row := range out.Items {
		assert.Equal(t, u.ID, row.ActorID)
		assert.Equal(t, "Test Admin", row.ActorName)
	}
	assert.Equal(t, 401, request(t, router, "GET", "/api/v1/audit-logs", "", nil).Code)
}
