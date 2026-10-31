package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sea-codex-server/internal/audit/dto"
	"sea-codex-server/internal/audit/migrations"
	"sea-codex-server/internal/audit/models"
	"sea-codex-server/internal/audit/repositories"
	am "sea-codex-server/internal/auth/migrations"
	um "sea-codex-server/internal/auth/models"
	"sea-codex-server/internal/common"
)

func TestActorNamesAreReadOnlyAndTenantScoped(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	require.NoError(t, am.MigrateAuthModels(ctx, db))
	require.NoError(t, migrations.MigrateAuditModels(ctx, db))
	assert.False(t, db.Migrator().HasColumn(&models.AuditLog{}, "actor_name"), "display name must not add a database column")
	c1 := um.Company{Name: "Company A", Active: true}
	c2 := um.Company{Name: "Company B", Active: true}
	require.NoError(t, db.Create(&c1).Error)
	require.NoError(t, db.Create(&c2).Error)
	createUser := func(company uuid.UUID, username, name string, active bool) um.User {
		u := um.User{CompanyID: company, Username: username, Email: username + "@example.test", PasswordHash: "fixture-only", FullName: name, Role: "legal_specialist", Active: active}
		require.NoError(t, db.Create(&u).Error)
		return u
	}
	active := createUser(c1.ID, "active", "Nguyễn Văn Bình", true)
	fallback := createUser(c1.ID, "username-fallback", "  ", true)
	deleted := createUser(c1.ID, "deleted", "Historical Member", false)
	require.NoError(t, db.Delete(&deleted).Error)
	foreign := createUser(c2.ID, "foreign", "Other Company Secret Name", true)
	r := repositories.NewAuditRepository(db)
	for i, tc := range []struct {
		name string
		id   uuid.UUID
		want string
	}{
		{"active", active.ID, "Nguyễn Văn Bình"},
		{"username fallback", fallback.ID, "username-fallback"},
		{"inactive deleted", deleted.ID, "Historical Member"},
		{"foreign reference", foreign.ID, ""},
		{"missing actor", uuid.New(), ""},
		{"system", uuid.Nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource := uuid.New()
			dossier := uuid.New()
			now := time.Now().UTC().Add(time.Duration(i) * time.Second)
			event := models.AuditLog{CompanyID: c1.ID, ActorID: tc.id, ActorName: "forged display name", Action: "TEST", Resource: "dossier", ResourceID: resource, DossierID: &dossier, CreatedAt: now}
			require.NoError(t, r.Create(ctx, nil, &event))
			from, to := now.Add(-time.Millisecond), now.Add(time.Millisecond)
			out, err := r.List(ctx, c1.ID, dto.Filter{Page: common.Page{Limit: 2}, ActorID: &tc.id, ResourceID: &resource, DossierID: &dossier, Action: "TEST", From: &from, To: &to})
			require.NoError(t, err)
			require.Len(t, out.Items, 1)
			assert.EqualValues(t, 1, out.Total)
			assert.Equal(t, tc.want, out.Items[0].ActorName)
			assert.Equal(t, tc.id, out.Items[0].ActorID)
			var stored models.AuditLog
			require.NoError(t, db.First(&stored, "id = ?", event.ID).Error)
			assert.Empty(t, stored.ActorName)
		})
	}
	other := models.AuditLog{CompanyID: c2.ID, ActorID: foreign.ID, Action: "FOREIGN", Resource: "user", ResourceID: foreign.ID}
	require.NoError(t, r.Create(ctx, nil, &other))
	out, err := r.List(ctx, c1.ID, dto.Filter{Page: common.Page{Limit: 2, Offset: 2}})
	require.NoError(t, err)
	assert.EqualValues(t, 6, out.Total)
	assert.Len(t, out.Items, 2)
	_, err = r.List(ctx, uuid.Nil, dto.Filter{})
	assert.ErrorIs(t, err, common.ErrForbidden)
	require.NoError(t, pool.Close())
	_, err = r.List(ctx, c1.ID, dto.Filter{Page: common.Page{Limit: 2}})
	require.Error(t, err)
}
