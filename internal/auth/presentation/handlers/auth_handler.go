package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ad "sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/auth/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
)

type AuthHandler struct{ s services.IAuthService }

func NewAuthHandler(s services.IAuthService) *AuthHandler { return &AuthHandler{s: s} }
func respond(c *gin.Context, status int, message string, value any, err error) {
	c.Header("Cache-Control", "no-store")
	if err != nil {
		code, msg := common.HTTPError(err)
		dto.ErrorResponse(c, code, msg)
		return
	}
	dto.SuccessResponse(c, status, message, value)
}
func (h *AuthHandler) Signup(c *gin.Context) {
	var in ad.SignupRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.Signup(c.Request.Context(), in)
	respond(c, 201, "Company registered", out, err)
}
func (h *AuthHandler) Login(c *gin.Context) {
	var in ad.LoginRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.Login(c.Request.Context(), in)
	respond(c, 200, "Authenticated", out, err)
}
func (h *AuthHandler) Me(c *gin.Context) {
	out, err := h.s.Me(c.Request.Context())
	respond(c, 200, "Current account", out, err)
}
func (h *AuthHandler) ListUsers(c *gin.Context) {
	p, err := common.ParsePage(c.Query("limit"), c.Query("offset"))
	if err != nil {
		respond(c, 400, "", nil, err)
		return
	}
	out, err := h.s.ListUsers(c.Request.Context(), p)
	respond(c, 200, "Company members", out, err)
}
func (h *AuthHandler) CreateUser(c *gin.Context) {
	var in ad.CreateUserRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.CreateUser(c.Request.Context(), in)
	respond(c, 201, "Member created", out, err)
}
func (h *AuthHandler) UpdateAccess(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		dto.ErrorResponse(c, 400, "Invalid user ID")
		return
	}
	var in ad.UpdateAccessRequest
	if !dto.BindJSON(c, &in) {
		return
	}
	out, err := h.s.UpdateAccess(c.Request.Context(), id, in)
	respond(c, 200, "Member access updated", out, err)
}
