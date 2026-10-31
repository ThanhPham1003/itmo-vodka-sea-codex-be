package logic

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/auth/models"
	"sea-codex-server/internal/auth/repositories"
	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/token"
	"sea-codex-server/pkg/utils"
)

type AuthLogic struct {
	db        *gorm.DB
	users     repositories.IUserRepository
	companies repositories.ICompanyRepository
	tokens    token.ITokenManager
	audit     common.AuditWriter
	dummyHash string
}

func NewAuthLogic(db *gorm.DB, u repositories.IUserRepository, c repositories.ICompanyRepository, t token.ITokenManager, a common.AuditWriter) *AuthLogic {
	// A valid fixed-cost bcrypt hash, used only for failed-lookup timing, never a usable account.
	return &AuthLogic{db: db, users: u, companies: c, tokens: t, audit: a, dummyHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"}
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,63}$`)

func credentials(username, email, password, name string) (string, string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if !usernamePattern.MatchString(username) || err != nil || parsed.Address != email || len(email) > 254 || utf8.RuneCountInString(password) < 12 || len(password) > 72 || strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 200 {
		return "", "", common.ErrInvalidArgument
	}
	return username, email, nil
}
func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return fmt.Errorf("%w: account already exists", common.ErrConflict)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrNotFound
	}
	return fmt.Errorf("%w: %w", common.ErrUnavailable, err)
}
func (l *AuthLogic) Signup(ctx context.Context, in dto.SignupRequest) (dto.UserResponse, error) {
	u, e, err := credentials(in.Username, in.Email, in.Password, in.FullName)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if strings.TrimSpace(in.Company.Name) == "" || len(in.Company.Name) > 800 {
		return dto.UserResponse{}, common.ErrInvalidArgument
	}
	hash, err := utils.HashPassword(in.Password)
	if err != nil {
		return dto.UserResponse{}, common.ErrInvalidArgument
	}
	c := models.Company{Name: strings.TrimSpace(in.Company.Name), LegalName: in.Company.LegalName, BusinessRegistrationNumber: in.Company.BusinessRegistrationNumber, Address: in.Company.Address, Email: in.Company.Email, Phone: in.Company.Phone, Active: true}
	user := models.User{Username: u, Email: e, PasswordHash: hash, FullName: strings.TrimSpace(in.FullName), Role: common.RoleCompanyAdmin, Active: true, Permissions: []string{}}
	err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := l.companies.Create(ctx, tx, &c); err != nil {
			return err
		}
		user.CompanyID = c.ID
		if err := l.users.Create(ctx, tx, &user); err != nil {
			return err
		}
		return l.audit.Write(ctx, tx, common.AuditEvent{CompanyID: c.ID, ActorID: user.ID, Action: "COMPANY_REGISTERED", Resource: "company", ResourceID: c.ID})
	})
	if err != nil {
		return dto.UserResponse{}, persistenceError(err)
	}
	user.Company = c
	return dto.UserDTO(user, true), nil
}
func (l *AuthLogic) Login(ctx context.Context, in dto.LoginRequest) (dto.LoginResponse, error) {
	identifier := strings.ToLower(strings.TrimSpace(in.UsernameOrEmail))
	u, err := l.users.FindLogin(ctx, identifier)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.CheckPasswordHash(in.Password, l.dummyHash)
		return dto.LoginResponse{}, common.ErrUnauthenticated
	}
	if err != nil {
		return dto.LoginResponse{}, persistenceError(err)
	}
	if !utils.CheckPasswordHash(in.Password, u.PasswordHash) || !u.Active || u.Company.ID == uuid.Nil || !u.Company.Active {
		return dto.LoginResponse{}, common.ErrUnauthenticated
	}
	encoded, claims, err := l.tokens.CreateToken(ctx, u.ID)
	if err != nil {
		return dto.LoginResponse{}, fmt.Errorf("%w: %w", common.ErrUnavailable, err)
	}
	if err = l.audit.Write(ctx, nil, common.AuditEvent{CompanyID: u.CompanyID, ActorID: u.ID, Action: "LOGIN", Resource: "user", ResourceID: u.ID}); err != nil {
		return dto.LoginResponse{}, persistenceError(err)
	}
	return dto.LoginResponse{AccessToken: encoded, TokenType: "Bearer", ExpiresAt: claims.ExpiresAt, User: dto.UserDTO(u, false)}, nil
}
func (l *AuthLogic) ResolveIdentity(ctx context.Context, id uuid.UUID) (common.Actor, error) {
	u, err := l.users.FindIdentity(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.Actor{}, common.ErrUnauthenticated
	}
	if err != nil {
		return common.Actor{}, persistenceError(err)
	}
	if !u.Active || u.Company.ID == uuid.Nil || !u.Company.Active || !common.ValidCompanyRole(u.Role) {
		return common.Actor{}, common.ErrUnauthenticated
	}
	return common.Actor{UserID: u.ID, CompanyID: u.CompanyID, Email: u.Email, Role: u.Role, Permissions: common.EffectivePermissions(u.Role, u.Permissions)}, nil
}
func (l *AuthLogic) Me(ctx context.Context) (dto.UserResponse, error) {
	a, ok := common.ActorFromContext(ctx)
	if !ok {
		return dto.UserResponse{}, common.ErrUnauthenticated
	}
	u, err := l.users.FindIdentity(ctx, a.UserID)
	if err != nil {
		return dto.UserResponse{}, persistenceError(err)
	}
	if u.CompanyID != a.CompanyID || !u.Active || !u.Company.Active {
		return dto.UserResponse{}, common.ErrUnauthenticated
	}
	return dto.UserDTO(u, true), nil
}
func admin(ctx context.Context) (common.Actor, error) {
	a, ok := common.ActorFromContext(ctx)
	if !ok {
		return a, common.ErrUnauthenticated
	}
	if a.Role != common.RoleCompanyAdmin {
		return a, common.ErrForbidden
	}
	return a, nil
}
func validateAccess(role string, permissions []string) error {
	if !common.ValidCompanyRole(role) || len(permissions) > 64 {
		return common.ErrInvalidArgument
	}
	for _, p := range permissions {
		if !common.KnownPermission(p) {
			return common.ErrInvalidArgument
		}
		if role == "broker" && (p == "pii:view" || p == "dossier:image_view" || p == "dossier:correct" || p == "dossier:review") {
			return common.ErrInvalidArgument
		}
	}
	return nil
}
func (l *AuthLogic) ListUsers(ctx context.Context, p common.Page) (common.List[dto.UserResponse], error) {
	a, err := admin(ctx)
	out := common.List[dto.UserResponse]{Items: []dto.UserResponse{}, Page: p}
	if err != nil {
		return out, err
	}
	users, err := l.users.List(ctx, a.CompanyID, p)
	if err != nil {
		return out, persistenceError(err)
	}
	out.Total = users.Total
	for _, u := range users.Items {
		out.Items = append(out.Items, dto.UserDTO(u, false))
	}
	return out, nil
}
func (l *AuthLogic) CreateUser(ctx context.Context, in dto.CreateUserRequest) (dto.UserResponse, error) {
	a, err := admin(ctx)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if err = validateAccess(in.Role, in.Permissions); err != nil {
		return dto.UserResponse{}, err
	}
	u, e, err := credentials(in.Username, in.Email, in.Password, in.FullName)
	if err != nil {
		return dto.UserResponse{}, err
	}
	hash, err := utils.HashPassword(in.Password)
	if err != nil {
		return dto.UserResponse{}, common.ErrInvalidArgument
	}
	user := models.User{CompanyID: a.CompanyID, Username: u, Email: e, PasswordHash: hash, FullName: strings.TrimSpace(in.FullName), Role: in.Role, Permissions: in.Permissions, Active: true}
	err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := l.companies.Lock(ctx, tx, a.CompanyID); err != nil {
			return err
		}
		if err := l.users.Create(ctx, tx, &user); err != nil {
			return err
		}
		return l.audit.Write(ctx, tx, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "USER_CREATED", Resource: "user", ResourceID: user.ID, Metadata: map[string]any{"role": user.Role}})
	})
	if err != nil {
		return dto.UserResponse{}, persistenceError(err)
	}
	return dto.UserDTO(user, false), nil
}
func (l *AuthLogic) UpdateAccess(ctx context.Context, id uuid.UUID, in dto.UpdateAccessRequest) (dto.UserResponse, error) {
	a, err := admin(ctx)
	if err != nil {
		return dto.UserResponse{}, err
	}
	if in.Active == nil {
		return dto.UserResponse{}, common.ErrInvalidArgument
	}
	if err = validateAccess(in.Role, in.Permissions); err != nil {
		return dto.UserResponse{}, err
	}
	var user models.User
	err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := l.companies.Lock(ctx, tx, a.CompanyID); err != nil {
			return err
		}
		var err error
		user, err = l.users.FindInCompany(ctx, tx, a.CompanyID, id)
		if err != nil {
			return err
		}
		if user.Role == common.RoleCompanyAdmin && user.Active && (in.Role != common.RoleCompanyAdmin || !*in.Active) {
			var count int64
			if err = tx.Model(&models.User{}).Where("company_id = ? AND role = ? AND active = ?", a.CompanyID, common.RoleCompanyAdmin, true).Count(&count).Error; err != nil {
				return err
			}
			if count <= 1 {
				return common.ErrConflict
			}
		}
		user.Role = in.Role
		user.Permissions = in.Permissions
		user.Active = *in.Active
		if err = tx.Omit("Company").Save(&user).Error; err != nil {
			return err
		}
		return l.audit.Write(ctx, tx, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "USER_ACCESS_CHANGED", Resource: "user", ResourceID: user.ID, Metadata: map[string]any{"role": user.Role, "active": user.Active}})
	})
	if err != nil {
		if errors.Is(err, common.ErrConflict) {
			return dto.UserResponse{}, err
		}
		return dto.UserResponse{}, persistenceError(err)
	}
	return dto.UserDTO(user, false), nil
}
