// Package dossier owns catalogs, dossiers, intake, extraction, reconciliation, review and dashboard.
// This scaffold reserves wiring only; no business routes or services are installed.
package dossier

import "sea-codex-server/internal/common"

// RegisterServices is the module composition point for future concrete services.
func RegisterServices(_ *common.App) {}

// NewServer will mount approved business routes on app.Router.
func NewServer(_ *common.App) {}
