package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/ai"
)

type suggestionAI struct {
	raw json.RawMessage
	err error
}

func (s suggestionAI) Complete(context.Context, string, []ai.Image) (json.RawMessage, error) {
	return s.raw, s.err
}

type suggestionAudit struct{ err error }

func (s suggestionAudit) Write(context.Context, *gorm.DB, common.AuditEvent) error { return s.err }

func TestDocumentSuggestionFailureLogs(t *testing.T) {
	valid := json.RawMessage(`{"name":"Draft","description":"private-content","fields":[{"key":"","label":"Field","type":"text"}],"special_requirements":[]}`)
	for _, tc := range []struct {
		name, stage, reason string
		client              suggestionAI
		auditErr            error
	}{
		{"provider", "ai_request", "provider_http_error", suggestionAI{err: &ai.Failure{Reason: "provider_http_error", Provider: "openai-compatible", Model: "fixture", Timeout: 90 * time.Second, Attempt: 1, HTTPStatus: 400, ProviderCode: "unsupported_parameter"}}, nil},
		{"timeout", "ai_request", "timeout", suggestionAI{err: context.DeadlineExceeded}, nil},
		{"canceled", "ai_request", "canceled", suggestionAI{err: context.Canceled}, nil},
		{"unknown provider error", "ai_request", "dependency_unavailable", suggestionAI{err: errors.New("private-error private-key")}, nil},
		{"schema", "decode", "invalid_schema", suggestionAI{raw: json.RawMessage(`{"private-content":"private-key"}`)}, nil},
		{"validation", "validation", "invalid_definition", suggestionAI{raw: json.RawMessage(`{"name":"private-content","fields":[{"key":"generated","label":"Field","type":"text"}]}`)}, nil},
		{"audit", "audit", "dependency_unavailable", suggestionAI{raw: valid}, common.ErrUnavailable},
		{"success", "", "", suggestionAI{raw: valid}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := NewCatalogLogic(nil, suggestionAudit{tc.auditErr}, tc.client, zerolog.New(&buf))
			ctx := common.WithRequestID(common.WithActor(context.Background(), common.Actor{UserID: uuid.New(), CompanyID: uuid.New(), Role: common.RoleCompanyAdmin}), "fixture-request-id")
			_, err := l.SuggestDocument(ctx, "private-prompt")
			if tc.stage == "" {
				require.NoError(t, err)
				assert.Empty(t, buf.String())
				return
			}
			require.ErrorIs(t, err, common.ErrUnavailable)
			var entry map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, "error", entry["level"])
			assert.Equal(t, "catalog suggestion failed", entry["message"])
			assert.Equal(t, "fixture-request-id", entry["request_id"])
			assert.Equal(t, "document_type", entry["operation"])
			assert.Equal(t, tc.stage, entry["stage"])
			assert.Equal(t, tc.reason, entry["reason"])
			if tc.name == "provider" {
				assert.Equal(t, float64(400), entry["upstream_status"])
				assert.Equal(t, float64(90000), entry["timeout_ms"])
				assert.Equal(t, float64(1), entry["attempt"])
				assert.Equal(t, "fixture", entry["model"])
				assert.Equal(t, "unsupported_parameter", entry["provider_error_code"])
			}
			for _, private := range []string{"private-prompt", "private-content", "private-key", "private-error"} {
				assert.NotContains(t, buf.String(), private)
			}
		})
	}
}
