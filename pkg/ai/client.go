// Package ai implements bounded, tool-free JSON vision requests to configured providers.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sea-codex-server/pkg/config"
)

var ErrUnavailable = errors.New("AI provider unavailable")

type Image struct {
	MIME string
	Data []byte
}
type Client interface {
	Complete(context.Context, string, []Image) (json.RawMessage, error)
}

// StructuredClient can constrain a single completion without changing other
// callers (OCR, assistant, or other catalog suggestions).
type StructuredClient interface {
	Client
	CompleteJSON(context.Context, string, []Image, json.RawMessage) (json.RawMessage, error)
}
type HTTPClient struct {
	cfg             config.ProcessingConfig
	http            *http.Client
	maxOutputTokens int
	instructions    string
}

type Option func(*HTTPClient)

// WithMaxOutputTokens leaves the OCR default unchanged when omitted.
func WithMaxOutputTokens(n int) Option {
	return func(c *HTTPClient) {
		if n > 0 {
			c.maxOutputTokens = n
		}
	}
}

// WithInstructions separates trusted application policy from untrusted user data.
func WithInstructions(text string) Option { return func(c *HTTPClient) { c.instructions = text } }

func NewClient(cfg config.ProcessingConfig, options ...Option) *HTTPClient {
	c := &HTTPClient{cfg: cfg, maxOutputTokens: 8192, http: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	for _, option := range options {
		option(c)
	}
	return c
}
func (c *HTTPClient) Complete(ctx context.Context, prompt string, images []Image) (json.RawMessage, error) {
	return c.complete(ctx, prompt, images, nil)
}

func (c *HTTPClient) CompleteJSON(ctx context.Context, prompt string, images []Image, schema json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(schema) {
		return nil, c.failure("invalid_output_schema", 0, 0, nil)
	}
	// Anthropic's existing message contract is kept; the schema is provided as
	// guidance and the caller still validates its response locally.
	if c.cfg.Provider == "anthropic" {
		return c.Complete(ctx, prompt+"\nOutput JSON schema: "+string(schema), images)
	}
	return c.complete(ctx, prompt, images, schema)
}

func (c *HTTPClient) complete(ctx context.Context, prompt string, images []Image, schema json.RawMessage) (json.RawMessage, error) {
	if c.cfg.APIKey == "" || c.cfg.Model == "" {
		return nil, c.failure("missing_configuration", 0, 0, nil)
	}
	u, err := url.Parse(c.cfg.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, c.failure("invalid_base_url", 0, 0, nil)
	}
	if len(prompt) > 1024*1024 || len(images) > 100 {
		return nil, c.failure("request_limit_exceeded", 0, 0, nil)
	}
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	content := []map[string]any{}
	for _, img := range images {
		if (img.MIME != "image/png" && img.MIME != "image/jpeg") || len(img.Data) > 20*1024*1024 {
			return nil, c.failure("invalid_image", 0, 0, nil)
		}
		encoded := base64.StdEncoding.EncodeToString(img.Data)
		if c.cfg.Provider == "anthropic" {
			content = append(content, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": img.MIME, "data": encoded}})
		} else {
			content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + img.MIME + ";base64," + encoded}})
		}
	}
	content = append(content, map[string]any{"type": "text", "text": prompt})
	body := map[string]any{"model": c.cfg.Model, "messages": []map[string]any{{"role": "user", "content": content}}}
	if c.instructions != "" && c.cfg.Provider != "anthropic" {
		body["messages"] = []map[string]any{{"role": "developer", "content": c.instructions}, {"role": "user", "content": content}}
	}
	endpoint := base + "/chat/completions"
	if c.cfg.Provider == "anthropic" {
		endpoint = base + "/messages"
		body["max_tokens"] = c.maxOutputTokens
	} else {
		body["max_completion_tokens"] = c.maxOutputTokens
		body["response_format"] = map[string]string{"type": "json_object"}
		if schema != nil {
			body["response_format"] = map[string]any{
				"type":        "json_schema",
				"json_schema": map[string]any{"name": "configuration", "strict": true, "schema": schema},
			}
		}
		body["store"] = false
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode AI request: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, c.failure("invalid_request", attempt+1, 0, nil)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.cfg.Provider == "anthropic" {
			req.Header.Set("x-api-key", c.cfg.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, c.failure(transportFailureReason(err), attempt+1, 0, nil)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
		res.Body.Close()
		if readErr != nil {
			reason := transportFailureReason(readErr)
			if reason == "network_error" {
				reason = "response_read_error"
			}
			return nil, c.failure(reason, attempt+1, res.StatusCode, nil)
		}
		if len(raw) > 2*1024*1024 {
			return nil, c.failure("response_too_large", attempt+1, res.StatusCode, nil)
		}
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			if attempt == 2 {
				return nil, c.failure("provider_http_error", attempt+1, res.StatusCode, raw)
			}
			delay := time.Duration(1<<attempt) * 250 * time.Millisecond
			if n, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && n > 0 && n <= 5 {
				delay = time.Duration(n) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if res.StatusCode != 200 {
			return nil, c.failure("provider_http_error", attempt+1, res.StatusCode, raw)
		}
		var text string
		if c.cfg.Provider == "anthropic" {
			var out struct {
				StopReason string `json:"stop_reason"`
				Content    []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if json.Unmarshal(raw, &out) != nil {
				return nil, c.failure("invalid_provider_response", attempt+1, res.StatusCode, nil)
			}
			if out.StopReason == "max_tokens" {
				return nil, c.failure("output_truncated", attempt+1, res.StatusCode, nil)
			}
			for _, part := range out.Content {
				if part.Type == "text" {
					text += part.Text
				}
			}
		} else {
			var out struct {
				Choices []struct {
					FinishReason string `json:"finish_reason"`
					Message      struct {
						Content string `json:"content"`
						Refusal string `json:"refusal"`
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal(raw, &out) != nil || len(out.Choices) != 1 {
				return nil, c.failure("invalid_provider_response", attempt+1, res.StatusCode, nil)
			}
			if out.Choices[0].Message.Refusal != "" {
				return nil, c.failure("provider_refusal", attempt+1, res.StatusCode, nil)
			}
			if out.Choices[0].FinishReason != "stop" {
				reason := "unexpected_finish_reason"
				if out.Choices[0].FinishReason == "length" {
					reason = "output_truncated"
				}
				return nil, c.failure(reason, attempt+1, res.StatusCode, nil)
			}
			text = out.Choices[0].Message.Content
		}
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "```json") {
			text = strings.TrimPrefix(text, "```json")
			text = strings.TrimSuffix(strings.TrimSpace(text), "```")
		}
		if !json.Valid([]byte(text)) {
			return nil, c.failure("invalid_json_content", attempt+1, res.StatusCode, nil)
		}
		return json.RawMessage(text), nil
	}
	return nil, ErrUnavailable
}
