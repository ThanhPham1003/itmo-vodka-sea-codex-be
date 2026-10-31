package ai_test

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderContractsAndRetries(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai-compatible"} {
		t.Run(provider, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				assert.NotContains(t, payload, "tools")
				assert.NotContains(t, payload, "functions")
				if calls.Add(1) == 1 {
					w.WriteHeader(429)
					return
				}
				if provider == "anthropic" {
					assert.Equal(t, "/v1/messages", r.URL.Path)
					assert.Equal(t, "fixture-key", r.Header.Get("x-api-key"))
					assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
					_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"{\"ok\":true}"}]}`))
				} else {
					assert.Equal(t, "/v1/chat/completions", r.URL.Path)
					assert.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
					_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}"}}]}`))
				}
			}))
			defer server.Close()
			client := ai.NewClient(config.ProcessingConfig{Provider: provider, BaseURL: server.URL, APIKey: "fixture-key", Model: "fixture-model", Timeout: time.Second})
			out, err := client.Complete(context.Background(), "Return JSON", nil)
			require.NoError(t, err)
			assert.JSONEq(t, `{"ok":true}`, string(out))
			assert.EqualValues(t, 2, calls.Load())
		})
	}
}
func TestProviderFailuresAreSafe(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"denied", 401, "private-provider-response"}, {"invalid JSON content", 200, `{"choices":[{"message":{"content":"not JSON"}}]}`}, {"truncated", 200, `{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := ai.NewClient(config.ProcessingConfig{Provider: "openai-compatible", BaseURL: server.URL, APIKey: "secret", Model: "fixture", Timeout: time.Second})
			_, err := client.Complete(context.Background(), "JSON", nil)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret")
			assert.NotContains(t, err.Error(), "private-provider-response")
		})
	}
}

func TestAssistantOptionsKeepOCRDefaults(t *testing.T) {
	for _, assistant := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload struct {
				Max      int `json:"max_completion_tokens"`
				Messages []struct {
					Role string `json:"role"`
				} `json:"messages"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			if assistant {
				assert.Equal(t, 1500, payload.Max)
				require.Len(t, payload.Messages, 2)
				assert.Equal(t, "developer", payload.Messages[0].Role)
			} else {
				assert.Equal(t, 8192, payload.Max)
				assert.Len(t, payload.Messages, 1)
			}
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}]}`))
		}))
		cfg := config.ProcessingConfig{Provider: "openai-compatible", BaseURL: server.URL, APIKey: "test", Model: "test", Timeout: time.Second}
		options := []ai.Option{}
		if assistant {
			options = append(options, ai.WithMaxOutputTokens(1500), ai.WithInstructions("Return JSON only"))
		}
		_, err := ai.NewClient(cfg, options...).Complete(context.Background(), "untrusted data", nil)
		require.NoError(t, err)
		server.Close()
	}
}
