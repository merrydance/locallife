package baofu

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type FrontendGuidance struct {
	Code      string
	Message   string
	Action    string
	Retryable bool
}

type ProviderError struct {
	Operation          string
	Capability         string
	StatusCode         int
	RequestID          string
	UpstreamCode       string
	UpstreamMessage    string
	DiagnosticSnapshot []byte
	Frontend           FrontendGuidance
	cause              error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "baofu provider error"
	}
	return "baofu " + strings.TrimSpace(e.Operation) + " failed: code=" + strings.TrimSpace(e.UpstreamCode)
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

type RequestMetadata struct {
	Capability    string
	OutRequestNo  string
	OutTradeNo    string
	HTTPStatus    int
	UpstreamCode  string
	RequestID     string
	IDCardNo      string
	BankCardNo    string
	Mobile        string
	PrivateKeyPEM string
}

func (m RequestMetadata) SafeLogFields() map[string]any {
	fields := map[string]any{"provider": "baofu"}
	if v := strings.TrimSpace(m.Capability); v != "" {
		fields["capability"] = v
	}
	if v := strings.TrimSpace(m.OutRequestNo); v != "" {
		fields["out_request_no"] = v
	}
	if v := strings.TrimSpace(m.OutTradeNo); v != "" {
		fields["out_trade_no"] = v
	}
	if m.HTTPStatus != 0 {
		fields["http_status"] = m.HTTPStatus
	}
	if v := strings.TrimSpace(m.UpstreamCode); v != "" {
		fields["upstream_code"] = v
	}
	if v := strings.TrimSpace(m.RequestID); v != "" {
		fields["request_id"] = v
	}
	return fields
}

func LogProviderError(logger zerolog.Logger, err error, metadata RequestMetadata) {
	if err == nil {
		return
	}
	event := logger.Error().Err(err)
	for key, value := range metadata.SafeLogFields() {
		event = event.Interface(key, value)
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		event = event.
			Str("operation", strings.TrimSpace(providerErr.Operation)).
			Str("capability", strings.TrimSpace(providerErr.Capability)).
			Str("request_id", strings.TrimSpace(providerErr.RequestID)).
			Str("upstream_code", strings.TrimSpace(providerErr.UpstreamCode)).
			Str("frontend_code", strings.TrimSpace(providerErr.Frontend.Code)).
			Bool("retryable", providerErr.Frontend.Retryable)
		if providerErr.StatusCode != 0 {
			event = event.Int("http_status", providerErr.StatusCode)
		}
		if upstreamMessage := SanitizeUpstreamMessageForRecord(providerErr.UpstreamMessage); upstreamMessage != "" {
			event = event.Str("upstream_message_sanitized", upstreamMessage)
		}
	}
	event.Msg("baofu request failed")
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Transport struct {
	client  HTTPDoer
	timeout time.Duration
}

func NewTransport(client HTTPDoer, timeout time.Duration) *Transport {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &Transport{client: client, timeout: timeout}
}
