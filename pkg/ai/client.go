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
type HTTPClient struct {
	cfg  config.ProcessingConfig
	http *http.Client
}

func NewClient(cfg config.ProcessingConfig) *HTTPClient {
	return &HTTPClient{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *HTTPClient) Complete(ctx context.Context, prompt string, images []Image) (json.RawMessage, error) {
	if c.cfg.APIKey == "" || c.cfg.Model == "" {
		return nil, ErrUnavailable
	}
	u, err := url.Parse(c.cfg.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, ErrUnavailable
	}
	if len(prompt) > 1024*1024 || len(images) > 100 {
		return nil, ErrUnavailable
	}
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	content := []map[string]any{}
	for _, img := range images {
		if (img.MIME != "image/png" && img.MIME != "image/jpeg") || len(img.Data) > 20*1024*1024 {
			return nil, ErrUnavailable
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
	endpoint := base + "/chat/completions"
	if c.cfg.Provider == "anthropic" {
		endpoint = base + "/messages"
		body["max_tokens"] = 8192
	} else {
		body["max_completion_tokens"] = 8192
		body["response_format"] = map[string]string{"type": "json_object"}
		body["store"] = false
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode AI request: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, ErrUnavailable
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
			return nil, ErrUnavailable
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
		res.Body.Close()
		if readErr != nil || len(raw) > 2*1024*1024 {
			return nil, ErrUnavailable
		}
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			if attempt == 2 {
				return nil, ErrUnavailable
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
			return nil, ErrUnavailable
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
			if json.Unmarshal(raw, &out) != nil || out.StopReason == "max_tokens" {
				return nil, ErrUnavailable
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
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal(raw, &out) != nil || len(out.Choices) != 1 || out.Choices[0].FinishReason == "length" {
				return nil, ErrUnavailable
			}
			text = out.Choices[0].Message.Content
		}
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "```json") {
			text = strings.TrimPrefix(text, "```json")
			text = strings.TrimSuffix(strings.TrimSpace(text), "```")
		}
		if !json.Valid([]byte(text)) {
			return nil, ErrUnavailable
		}
		return json.RawMessage(text), nil
	}
	return nil, ErrUnavailable
}
