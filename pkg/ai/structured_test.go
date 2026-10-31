package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
)

func TestAnthropicStructuredCompletionUsesExistingContract(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		assert.NotContains(t, payload, "response_format")
		assert.Contains(t, string(payload["messages"]), "Output JSON schema:")
		_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"{\"ok\":true}"}]}`))
	}))
	defer server.Close()
	client := ai.NewClient(config.ProcessingConfig{Provider: "anthropic", BaseURL: server.URL, APIKey: "test", Model: "test", Timeout: time.Second})
	out, err := client.CompleteJSON(context.Background(), "Return JSON", nil, schema)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(out))
}

func TestInvalidOutputSchemaFailsBeforeRequest(t *testing.T) {
	client := ai.NewClient(config.ProcessingConfig{Provider: "openai-compatible", BaseURL: "https://invalid.example", APIKey: "test", Model: "test", Timeout: time.Second})
	_, err := client.CompleteJSON(context.Background(), "Return JSON", nil, json.RawMessage(`{"type":`))
	var failure *ai.Failure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, "invalid_output_schema", failure.Reason)
	assert.Zero(t, failure.Attempt)
}
