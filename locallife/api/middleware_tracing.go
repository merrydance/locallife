package api

import (
	"context"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// contextKeyType is a private type for context keys to avoid collisions
type contextKeyType string

const (
	// RequestIDHeader HTTP 请求头中的 request_id 键
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey Gin Context 中的 request_id 键
	RequestIDKey    = "request_id"
	requestIDCtxKey = contextKeyType("request_id")
)

// RequestTracingMiddleware 请求追踪中间件
// 为每个请求生成唯一的 request_id，注入到日志和响应头
func RequestTracingMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 检查请求头中是否已有 request_id（可能由网关注入）
		requestID := ctx.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// 存储到 Context 中供后续使用
		ctx.Set(RequestIDKey, requestID)
		ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), requestIDCtxKey, requestID))

		// 设置响应头
		ctx.Header(RequestIDHeader, requestID)

		// 继续处理请求
		ctx.Next()
	}
}

// RequestLoggingMiddleware 请求日志中间件
// 记录每个请求的详细信息，包含 request_id
func RequestLoggingMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		query := sanitizeQuery(ctx.Request.URL.RawQuery)

		// 获取 request_id
		requestID, _ := ctx.Get(RequestIDKey)

		// 处理请求
		ctx.Next()

		// 计算耗时
		latency := time.Since(start)
		status := ctx.Writer.Status()
		clientIP := ctx.ClientIP()
		method := ctx.Request.Method
		userAgent := ctx.Request.UserAgent()
		isClientLogScanner401 := path == "/v1/logs/error" && status == 401 && isScannerUserAgent(userAgent)

		// 根据状态码选择日志级别
		var logEvent *zerolog.Event
		switch {
		case isClientLogScanner401:
			logEvent = log.Info()
		case status >= 500:
			logEvent = log.Error()
		case status >= 400:
			logEvent = log.Warn()
		default:
			logEvent = log.Info()
		}

		// 构建日志
		logEvent.
			Str("request_id", requestID.(string)).
			Str("method", method).
			Str("path", path).
			Str("query", query).
			Int("status", status).
			Dur("latency", latency).
			Str("client_ip", clientIP).
			Str("user_agent", userAgent).
			Int("body_size", ctx.Writer.Size())

		// 如果有错误，记录错误信息
		if len(ctx.Errors) > 0 {
			logEvent.Str("errors", ctx.Errors.String())
		}

		// 如果有认证用户，记录用户ID
		if payload, exists := ctx.Get(authorizationPayloadKey); exists {
			userPayload := payload.(*token.Payload)
			logEvent.Int64("user_id", userPayload.UserID)
		}

		logEvent.Msg("HTTP request")
	}
}

func sanitizeQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "<redacted>"
	}
	for _, key := range []string{"token", "access_token", "refresh_token"} {
		if _, ok := values[key]; ok {
			values.Set(key, "***")
		}
	}
	return values.Encode()
}

// GetRequestID 从 Context 获取 request_id
func GetRequestID(ctx *gin.Context) string {
	if requestID, exists := ctx.Get(RequestIDKey); exists {
		return requestID.(string)
	}
	return ""
}
