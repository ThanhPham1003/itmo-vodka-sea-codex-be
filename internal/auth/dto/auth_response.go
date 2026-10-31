package dto

import (
	"github.com/google/uuid"
	"sea-codex-server/internal/auth/models"
	"sea-codex-server/internal/common"
	"time"
)

type UserResponse struct {
	ID          uuid.UUID       `json:"id"`
	CompanyID   uuid.UUID       `json:"company_id"`
	Username    string          `json:"username"`
	Email       string          `json:"email"`
	FullName    string          `json:"full_name"`
	Role        string          `json:"role"`
	Permissions []string        `json:"permissions"`
	Active      bool            `json:"active"`
	Company     *models.Company `json:"company,omitempty"`
}

func UserDTO(u models.User, company bool) UserResponse {
	r := UserResponse{ID: u.ID, CompanyID: u.CompanyID, Username: u.Username, Email: u.Email, FullName: u.FullName, Role: u.Role, Permissions: common.EffectivePermissions(u.Role, u.Permissions), Active: u.Active}
	if company {
		r.Company = &u.Company
	}
	return r
}

type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        UserResponse `json:"user"`
}
