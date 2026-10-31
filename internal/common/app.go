// Package common defines shared application dependencies and request identity.
package common

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"sea-codex-server/pkg/config"
	"sea-codex-server/pkg/token"
)

// App contains process dependencies only. Request-specific identity lives in context.
type App struct {
	Config           *config.Config
	DB               *gorm.DB
	Logger           zerolog.Logger
	TokenManager     token.ITokenManager
	IdentityResolver IdentityResolver
	Router           *gin.Engine
}
