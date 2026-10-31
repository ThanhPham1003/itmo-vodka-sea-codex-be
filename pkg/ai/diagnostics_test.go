package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
)

func TestFailureDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, provider, body, reason, code string
		status, attempts                   int
	}{
		{"unauthorized", "openai-compatible", `{"error":{"code":"invalid_api_key","message":"private-key private-prompt"}}`, "provider_http_error", "invalid_api_key", 401, 1},
		{"unsupported parameter", "openai-compatible", `{"error":{"code":"unsupported_parameter","type":"invalid_request_error"}}`, "provider_http_error", "unsupported_parameter", 400, 1},
		{"unknown code is omitted", "openai-compatible", `{"error":{"code":"private-key","type":"private-prompt"}}`, "provider_http_error", "", 400, 1},
		{"retry exhaustion", "openai-compatible", `{"error":{"type":"rate_limit_error"}}`, "provider_http_error", "rate_limit_error", 429, 3},
		{"provider unavailable", "anthropic", `{"error":{"type":"overloaded_error"}}`, "provider_http_error", "overloaded_error", 503, 3},
		{"malformed envelope", "openai-compatible", `private-response`, "invalid_provider_response", "", 200, 1},
		{"invalid content", "openai-compatible", `{"choices":[{"finish_reason":"stop","message":{"content":"private-response"}}]}`, "invalid_json_content", "", 200, 1},
		{"truncated", "openai-compatible", `{"choices":[{"finish_reason":"length","message":{"content":"private-response"}}]}`, "output_truncated", "", 200, 1},
		{"anthropic truncated", "anthropic", `{"stop_reason":"max_tokens","content":[]}`, "output_truncated", "", 200, 1},
		{"refusal", "openai-compatible", `{"choices":[{"finish_reason":"stop","message":{"refusal":"private-response"}}]}`, "provider_refusal", "", 200, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := ai.NewClient(config.ProcessingConfig{Provider: tc.provider, BaseURL: server.URL, APIKey: "private-key", Model: "fixture-model", Timeout: time.Second})
			_, err := client.Complete(context.Background(), "private-prompt", nil)
			require.ErrorIs(t, err, ai.ErrUnavailable)
			var failure *ai.Failure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, tc.reason, failure.Reason)
			assert.Equal(t, tc.status, failure.HTTPStatus)
			assert.Equal(t, tc.code, failure.ProviderCode)
			assert.Equal(t, tc.attempts, failure.Attempt)
			assert.EqualValues(t, tc.attempts, calls.Load())
			assert.Equal(t, time.Second, failure.Timeout)
			raw, marshalErr := json.Marshal(failure)
			require.NoError(t, marshalErr)
			for _, private := range []string{"private-key", "private-prompt", "private-response", server.URL} {
				assert.NotContains(t, string(raw), private)
				assert.NotContains(t, err.Error(), private)
			}
		})
	}
}

func TestTimeoutDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()
	client := ai.NewClient(config.ProcessingConfig{Provider: "openai-compatible", BaseURL: server.URL, APIKey: "private-key", Model: "fixture", Timeout: 50 * time.Millisecond})
	_, err := client.Complete(context.Background(), "JSON", nil)
	var failure *ai.Failure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, "timeout", failure.Reason)
	assert.Equal(t, 50*time.Millisecond, failure.Timeout)
	assert.Equal(t, 1, failure.Attempt)
}
