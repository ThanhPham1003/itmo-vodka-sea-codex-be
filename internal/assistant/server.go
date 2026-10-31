// Package assistant provides read-only, grounded product guidance, not dossier tools.
package assistant

import (
	"fmt"
	"sea-codex-server/internal/assistant/logic"
	"sea-codex-server/internal/assistant/repositories"
	"sea-codex-server/internal/assistant/routes"
	"sea-codex-server/internal/assistant/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
)

func RegisterServices(app *common.App) error {
	repo, err := repositories.NewKnowledgeRepository()
	if err != nil {
		return fmt.Errorf("initialize assistant knowledge: %w", err)
	}
	cfg := app.Config.Assistant
	client := ai.NewClient(config.ProcessingConfig{Provider: "openai-compatible", BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model, Timeout: cfg.Timeout}, ai.WithMaxOutputTokens(cfg.MaxOutputTokens), ai.WithInstructions(logic.Instructions))
	services.RegisterAssistantService(logic.NewAssistantLogic(repo, client, app.AuditWriter, app.DB, cfg))
	return nil
}
func NewServer(app *common.App) { routes.RegisterRoutes(app, services.AssistantService()) }
