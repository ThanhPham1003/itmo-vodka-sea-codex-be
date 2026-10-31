package logic

import (
	"context"
	"errors"

	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/ai"
)

func (l *CatalogLogic) logSuggestionFailure(ctx context.Context, operation, stage string, err error) {
	reason := "dependency_unavailable"
	switch stage {
	case "decode":
		reason = "invalid_schema"
	case "validation":
		reason = "invalid_definition"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		reason = "timeout"
	} else if errors.Is(err, context.Canceled) {
		reason = "canceled"
	}
	event := l.log.Error().Str("request_id", common.RequestIDFromContext(ctx)).
		Str("operation", operation).Str("stage", stage)
	if stage == "decode" {
		addDecodeDiagnostics(event, err)
	}
	var failure *ai.Failure
	if errors.As(err, &failure) {
		reason = failure.Reason
		event.Str("provider", failure.Provider).Str("model", failure.Model).
			Int64("timeout_ms", failure.Timeout.Milliseconds()).Int("attempt", failure.Attempt).
			Int("upstream_status", failure.HTTPStatus)
		if failure.ProviderCode != "" {
			event.Str("provider_error_code", failure.ProviderCode)
		}
	}
	// Never log err.Error(): parsing, transport and audit errors may include
	// generated content, request URLs, credentials, or database values.
	event.Str("reason", reason).Msg("catalog suggestion failed")
}
