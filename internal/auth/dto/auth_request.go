package dto

type CompanyRequest struct {
	Name                       string `json:"name" binding:"required,max=200"`
	LegalName                  string `json:"legal_name" binding:"max=300"`
	BusinessRegistrationNumber string `json:"business_registration_number" binding:"max=100"`
	Address                    string `json:"address" binding:"max=1000"`
	Email                      string `json:"email" binding:"omitempty,email,max=254"`
	Phone                      string `json:"phone" binding:"max=30"`
}
type SignupRequest struct {
	Company  CompanyRequest `json:"company" binding:"required"`
	Username string         `json:"username" binding:"required,min=3,max=64"`
	Email    string         `json:"email" binding:"required,email,max=254"`
	Password string         `json:"password" binding:"required,min=12,max=72"`
	FullName string         `json:"full_name" binding:"required,max=200"`
}
type LoginRequest struct {
	UsernameOrEmail string `json:"username_or_email" binding:"required,max=254"`
	Password        string `json:"password" binding:"required,max=72"`
}
type CreateUserRequest struct {
	Username    string   `json:"username" binding:"required,min=3,max=64"`
	Email       string   `json:"email" binding:"required,email,max=254"`
	Password    string   `json:"password" binding:"required,min=12,max=72"`
	FullName    string   `json:"full_name" binding:"required,max=200"`
	Role        string   `json:"role" binding:"required"`
	Permissions []string `json:"permissions"`
}
type UpdateAccessRequest struct {
	Role        string   `json:"role" binding:"required"`
	Permissions []string `json:"permissions"`
	Active      *bool    `json:"active" binding:"required"`
}
