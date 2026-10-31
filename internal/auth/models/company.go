package models

import "sea-codex-server/internal/common/models"

type Company struct {
	models.BaseModel
	Name                       string `gorm:"not null;size:200" json:"name"`
	LegalName                  string `gorm:"size:300" json:"legal_name"`
	BusinessRegistrationNumber string `gorm:"size:100" json:"business_registration_number"`
	Address                    string `gorm:"size:1000" json:"address"`
	Email                      string `gorm:"size:254" json:"email"`
	Phone                      string `gorm:"size:30" json:"phone"`
	Active                     bool   `gorm:"not null" json:"active"`
}
