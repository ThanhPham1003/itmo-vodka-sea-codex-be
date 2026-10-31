// Package auth owns company registration, users, authentication and membership.
// This scaffold reserves wiring only; no business routes or services are installed.
package auth

import "sea-codex-server/internal/common"

// RegisterServices is the module composition point for future concrete services.
func RegisterServices(_ *common.App) {}

// NewServer will mount approved business routes on app.Router.
func NewServer(_ *common.App) {}
