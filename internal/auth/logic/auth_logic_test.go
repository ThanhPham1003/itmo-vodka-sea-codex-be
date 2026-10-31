package logic_test

import (
	"context"
	"errors"
	"testing"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	al "sea-codex-server/internal/audit/logic"
	am "sea-codex-server/internal/audit/migrations"
	ar "sea-codex-server/internal/audit/repositories"
	ad "sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/auth/logic"
	"sea-codex-server/internal/auth/migrations"
	"sea-codex-server/internal/auth/models"
	"sea-codex-server/internal/auth/repositories"
	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/token"
)

func setup(t *testing.T) (*gorm.DB, *logic.AuthLogic) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_foreign_keys=1"), &gorm.Config{TranslateError: true, Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, migrations.MigrateAuthModels(context.Background(), db))
	require.NoError(t, am.MigrateAuditModels(context.Background(), db))
	key := paseto.NewV4AsymmetricSecretKey()
	tokens, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	s := logic.NewAuthLogic(db, repositories.NewUserRepository(db), repositories.NewCompanyRepository(), tokens, al.NewAuditLogic(ar.NewAuditRepository(db)))
	return db, s
}
func signup(username string) ad.SignupRequest {
	return ad.SignupRequest{Company: ad.CompanyRequest{Name: "Test company"}, Username: username, Email: username + "@example.test", Password: "a-valid-test-password", FullName: "Test Admin"}
}
func TestAuthAndTenantManagement(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()
	admin, err := s.Signup(ctx, signup("admin-one"))
	require.NoError(t, err)
	assert.Equal(t, "company_admin", admin.Role)
	_, err = s.Signup(ctx, signup("admin-one"))
	assert.ErrorIs(t, err, common.ErrConflict)
	var count int64
	require.NoError(t, db.Model(&models.Company{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	for _, identifier := range []string{"admin-one", "ADMIN-ONE@EXAMPLE.TEST"} {
		result, err := s.Login(ctx, ad.LoginRequest{UsernameOrEmail: identifier, Password: "a-valid-test-password"})
		require.NoError(t, err)
		assert.NotEmpty(t, result.AccessToken)
	}
	for _, in := range []ad.LoginRequest{{UsernameOrEmail: "missing", Password: "a-valid-test-password"}, {UsernameOrEmail: "admin-one", Password: "wrong-password"}} {
		_, err = s.Login(ctx, in)
		assert.ErrorIs(t, err, common.ErrUnauthenticated)
	}
	a, err := s.ResolveIdentity(ctx, admin.ID)
	require.NoError(t, err)
	adminCtx := common.WithActor(ctx, a)
	_, err = s.CreateUser(adminCtx, ad.CreateUserRequest{Username: "bad-admin", Email: "bad@example.test", Password: "a-valid-test-password", FullName: "Invalid", Role: "platform_admin"})
	assert.ErrorIs(t, err, common.ErrInvalidArgument)
	member, err := s.CreateUser(adminCtx, ad.CreateUserRequest{Username: "broker-one", Email: "broker@example.test", Password: "a-valid-test-password", FullName: "Broker", Role: "broker"})
	require.NoError(t, err)
	broker, err := s.ResolveIdentity(ctx, member.ID)
	require.NoError(t, err)
	_, err = s.ListUsers(common.WithActor(ctx, broker), common.Page{Limit: 20})
	assert.ErrorIs(t, err, common.ErrForbidden)
	other, err := s.Signup(ctx, signup("other-admin"))
	require.NoError(t, err)
	active := true
	_, err = s.UpdateAccess(adminCtx, other.ID, ad.UpdateAccessRequest{Role: "broker", Active: &active})
	assert.ErrorIs(t, err, common.ErrNotFound)
	_, err = s.UpdateAccess(adminCtx, admin.ID, ad.UpdateAccessRequest{Role: "broker", Active: &active})
	assert.ErrorIs(t, err, common.ErrConflict)
	active = false
	_, err = s.UpdateAccess(adminCtx, member.ID, ad.UpdateAccessRequest{Role: "broker", Active: &active})
	require.NoError(t, err)
	_, err = s.ResolveIdentity(ctx, member.ID)
	assert.ErrorIs(t, err, common.ErrUnauthenticated)
	_, err = s.Login(ctx, ad.LoginRequest{UsernameOrEmail: "broker-one", Password: "a-valid-test-password"})
	assert.ErrorIs(t, err, common.ErrUnauthenticated)
	users, err := s.ListUsers(adminCtx, common.Page{Limit: 20})
	require.NoError(t, err)
	assert.Len(t, users.Items, 2)
}

type failingAudit struct{}

func (failingAudit) Write(context.Context, *gorm.DB, common.AuditEvent) error {
	return errors.New("audit unavailable")
}
func TestSignupRollbackOnAuditFailure(t *testing.T) {
	db, _ := setup(t)
	s := logic.NewAuthLogic(db, repositories.NewUserRepository(db), repositories.NewCompanyRepository(), nil, failingAudit{})
	_, err := s.Signup(context.Background(), signup("rollback-admin"))
	assert.ErrorIs(t, err, common.ErrUnavailable)
	for _, model := range []any{&models.Company{}, &models.User{}} {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		assert.Zero(t, count)
	}
}
