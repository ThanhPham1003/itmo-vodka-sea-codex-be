package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
)

// Failure contains diagnostic metadata only. It never retains request/response
// bodies, credentials, URLs, or raw transport errors that could contain them.
type Failure struct {
	Reason       string
	Provider     string
	Model        string
	Timeout      time.Duration
	Attempt      int
	HTTPStatus   int
	ProviderCode string
}

func (e *Failure) Error() string { return ErrUnavailable.Error() + ": " + e.Reason }
func (e *Failure) Unwrap() error { return ErrUnavailable }

func (c *HTTPClient) failure(reason string, attempt, status int, raw []byte) error {
	return &Failure{
		Reason: reason, Provider: c.cfg.Provider, Model: c.cfg.Model,
		Timeout: c.cfg.Timeout, Attempt: attempt, HTTPStatus: status,
		ProviderCode: providerErrorCode(raw),
	}
}

func transportFailureReason(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return "timeout"
	}
	return "network_error"
}

func providerErrorCode(raw []byte) string {
	var response struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return ""
	}
	// Only known codes are logged: provider messages and arbitrary error fields
	// can echo prompts, credentials, or other sensitive data.
	for _, code := range []string{response.Error.Code, response.Error.Type} {
		switch code {
		case "invalid_api_key", "authentication_error", "permission_error", "permission_denied",
			"model_not_found", "insufficient_quota", "rate_limit_exceeded", "rate_limit_error",
			"invalid_request_error", "unsupported_parameter", "unsupported_value",
			"context_length_exceeded", "content_policy_violation", "server_error", "api_error", "overloaded_error":
			return code
		}
	}
	return ""
}
