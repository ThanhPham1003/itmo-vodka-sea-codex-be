package common_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/common"
)

func TestActorContextDoesNotShareMutablePermissions(t *testing.T) {
	actor := common.Actor{UserID: uuid.New(), CompanyID: uuid.New(), Permissions: []string{"dossier:read"}}
	ctx := common.WithActor(context.Background(), actor)
	actor.Permissions[0] = "dossier:review"
	stored, ok := common.ActorFromContext(ctx)
	require.True(t, ok)
	assert.True(t, stored.HasPermission("dossier:read"))
	assert.False(t, stored.HasPermission("dossier:review"))
	stored.Permissions[0] = "dossier:review"
	again, ok := common.ActorFromContext(ctx)
	require.True(t, ok)
	assert.True(t, again.HasPermission("dossier:read"))
	_, ok = common.ActorFromContext(context.Background())
	assert.False(t, ok)
}
