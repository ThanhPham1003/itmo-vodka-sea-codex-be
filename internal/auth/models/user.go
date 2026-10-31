package models

import (
	"github.com/google/uuid"
	base "sea-codex-server/internal/common/models"
)

type User struct {
	base.BaseModel
	CompanyID    uuid.UUID `gorm:"type:uuid;not null;index" json:"company_id"`
	Company      Company   `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT" json:"-"`
	Username     string    `gorm:"not null;size:64;uniqueIndex" json:"username"`
	Email        string    `gorm:"not null;size:254;uniqueIndex" json:"email"`
	PasswordHash string    `gorm:"not null;size:100" json:"-"`
	FullName     string    `gorm:"not null;size:200" json:"full_name"`
	Role         string    `gorm:"not null;size:50" json:"role"`
	Permissions  []string  `gorm:"serializer:json;type:jsonb" json:"permissions"`
	Active       bool      `gorm:"not null" json:"active"`
}
