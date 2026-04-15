package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// APIResponse is the unified response envelope for all JSON APIs.
//
// Convention:
// - Success: HTTP 2xx with {"code":0,"message":"ok","data":...}
// - Error:   HTTP 4xx/5xx with non-zero code and a human-readable message
//
// Note: We keep message intentionally short and safe for 5xx.
type APIResponse struct {
	Code    int             `json:"code" example:"0"`
	Message string          `json:"message" example:"ok"`
	Data    json.RawMessage `json:"data,omitempty"`
}

const (
	CodeOK             = 0
	CodeBadRequest     = 40000
	CodeUnauthorized   = 40100
	CodeForbidden      = 40300
	CodeNotFound       = 40400
	CodeConflict       = 40900
	CodeUnprocessable  = 42200
	CodeTooManyRequest = 42900
	CodeGatewayTimeout = 50400
	CodeInternalError  = 50000
	CodeBadGateway     = 50200
	CodeServiceUnavail = 50300
)

func statusToCode(status int) int {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusUnprocessableEntity:
		return CodeUnprocessable
	case http.StatusTooManyRequests:
		return CodeTooManyRequest
	case http.StatusBadGateway:
		return CodeBadGateway
	case http.StatusServiceUnavailable:
		return CodeServiceUnavail
	case http.StatusGatewayTimeout:
		return CodeGatewayTimeout
	default:
		if status >= 500 {
			return CodeInternalError
		}
		if status >= 400 {
			return CodeBadRequest
		}
		return CodeOK
	}
}

func isWebSocketUpgrade(c *gin.Context) bool {
	upgrade := strings.ToLower(c.GetHeader("Upgrade"))
	connection := strings.ToLower(c.GetHeader("Connection"))
	return strings.Contains(upgrade, "websocket") || strings.Contains(connection, "upgrade")
}

func isJSONContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "application/json")
}

func bodyLooksLikeWrapped(b []byte) bool {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return false
	}
	var probe struct {
		Code *int `json:"code"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return false
	}
	return probe.Code != nil
}

func extractErrorMessage(status int, body []byte) string {
	// For 500, always keep it safe. For 502/503/504, handlers may provide
	// a sanitized public message such as service-unavailable guidance.
	if status == http.StatusInternalServerError {
		return "internal server error"
	}

	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return http.StatusText(status)
	}

	// Try common shapes: {"error":"..."} or {"message":"..."}
	var probe struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &probe); err == nil {
		if probe.Error != "" {
			return probe.Error
		}
		if probe.Message != "" {
			return probe.Message
		}
	}

	return http.StatusText(status)
}

func sanitizeServerErrorMessage(status int, message string) string {
	trimmed := strings.TrimSpace(message)
	if status == http.StatusInternalServerError {
		return "internal server error"
	}
	if trimmed == "" {
		return http.StatusText(status)
	}

	normalized := strings.ToLower(trimmed)
	switch {
	case strings.Contains(normalized, "wechat ecommerce client is not configured"), strings.Contains(normalized, "ecommerce client not configured"):
		return "微信支付服务暂不可用，请稍后重试"
	case strings.Contains(normalized, "payment service not configured"), strings.Contains(normalized, "payment service not available"):
		return "支付服务暂不可用，请稍后重试"
	case strings.Contains(normalized, "media storage not configured"):
		return "媒体服务暂不可用，请稍后重试"
	case strings.Contains(normalized, "not configured"):
		return "服务暂不可用，请稍后重试"
	default:
		return trimmed
	}
}

func logRawServerErrorResponse(c *gin.Context, status int, originalMessage string, publicMessage string) {
	evt := log.Error().
		Str("request_id", GetRequestID(c)).
		Str("path", c.Request.URL.Path).
		Str("method", c.Request.Method).
		Int("status", status)

	if strings.TrimSpace(originalMessage) != "" {
		evt = evt.Str("original_message", originalMessage)
	}
	if strings.TrimSpace(publicMessage) != "" {
		evt = evt.Str("public_message", publicMessage)
	}

	evt.Msg("raw server error response wrapped by envelope")
}

type bodyCaptureWriter struct {
	gin.ResponseWriter
	statusCode  int
	wroteHeader bool
	body        bytes.Buffer
}

func (w *bodyCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteHeader = true
	// Do not write to the underlying writer yet.
}

func (w *bodyCaptureWriter) WriteHeaderNow() {
	w.wroteHeader = true
	// Intentionally do nothing: we buffer until the end.
}

func (w *bodyCaptureWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(data)
}

func (w *bodyCaptureWriter) WriteString(s string) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.WriteString(s)
}

func (w *bodyCaptureWriter) Status() int {
	if w.wroteHeader {
		return w.statusCode
	}
	return http.StatusOK
}

// ResponseEnvelopeMiddleware wraps all JSON responses into APIResponse.
// It is designed to minimize handler changes and enforce a stable client contract.
//
// It skips:
// - WebSocket upgrades
// - WeChat/Payment webhooks under /v1/webhooks (they have strict response format expectations)
func ResponseEnvelopeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Default on: wrap JSON responses unless client explicitly opts out.
		// Opt-out with: X-Response-Envelope: 0
		if c.GetHeader("X-Response-Envelope") == "0" {
			c.Next()
			return
		}

		if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
			c.Next()
			return
		}

		if isWebSocketUpgrade(c) {
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/v1/webhooks/") {
			c.Next()
			return
		}

		bw := &bodyCaptureWriter{ResponseWriter: c.Writer}
		c.Writer = bw
		c.Next()

		status := bw.Status()
		originalBody := bw.body.Bytes()

		// No body -> nothing to wrap.
		if len(bytes.TrimSpace(originalBody)) == 0 {
			// Still propagate status if handler set it explicitly.
			if bw.wroteHeader {
				bw.ResponseWriter.WriteHeader(status)
			}
			return
		}

		ct := bw.Header().Get("Content-Type")
		if !isJSONContentType(ct) {
			bw.ResponseWriter.WriteHeader(status)
			_, _ = bw.ResponseWriter.Write(originalBody)
			return
		}

		// If already wrapped, pass through.
		if bodyLooksLikeWrapped(originalBody) {
			bw.ResponseWriter.WriteHeader(status)
			_, _ = bw.ResponseWriter.Write(originalBody)
			return
		}

		var resp APIResponse
		if status >= 200 && status < 300 {
			resp.Code = CodeOK
			resp.Message = "ok"
			resp.Data = json.RawMessage(originalBody)
		} else {
			resp.Code = statusToCode(status)
			rawMessage := extractErrorMessage(status, originalBody)
			if status >= 500 {
				resp.Message = sanitizeServerErrorMessage(status, rawMessage)
				logRawServerErrorResponse(c, status, rawMessage, resp.Message)
			} else {
				resp.Message = rawMessage
			}
			// For 4xx, keep original error payload in data for debugging.
			if status >= 400 && status < 500 {
				resp.Data = json.RawMessage(originalBody)
			}
		}

		finalBytes, err := json.Marshal(resp)
		if err != nil {
			// Fallback to a safe internal error.
			fallback := APIResponse{Code: CodeInternalError, Message: "internal server error"}
			finalBytes, _ = json.Marshal(fallback)
			status = http.StatusInternalServerError
		}

		bw.Header().Set("Content-Type", "application/json; charset=utf-8")
		bw.ResponseWriter.WriteHeader(status)
		_, _ = bw.ResponseWriter.Write(finalBytes)
	}
}
