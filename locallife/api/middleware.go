package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

const (
	authorizationHeaderKey  = "authorization"
	authorizationTypeBearer = "bearer"
	authorizationPayloadKey = "authorization_payload"
)

// AuthMiddleware creates a gin middleware for authorization
func authMiddleware(tokenMaker token.Maker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var accessToken string
		authorizationHeader := ctx.GetHeader(authorizationHeaderKey)

		if len(authorizationHeader) != 0 {
			fields := strings.Fields(authorizationHeader)
			if len(fields) >= 2 && strings.ToLower(fields[0]) == authorizationTypeBearer {
				accessToken = fields[1]
			}
		}

		if len(accessToken) == 0 && isWebSocketUpgrade(ctx) {
			accessToken = ctx.Query("token")
		}

		if len(accessToken) == 0 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("access token is not provided")))
			return
		}

		payload, err := tokenMaker.VerifyToken(accessToken, token.TokenTypeAccessToken)
		if err != nil {
			if isWebSocketUpgrade(ctx) {
				log.Warn().
					Err(err).
					Str("url", ctx.Request.URL.String()).
					Msg("WebSocket authentication failed")
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(err))
			return
		}

		ctx.Set(authorizationPayloadKey, payload)
		ctx.Next()
	}
}

// TimeoutMiddleware 为所有请求设置统一超时时间
// 防止慢查询、外部API卡死导致goroutine泄漏
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ⚠️ 注意：不要在 goroutine 里调用 c.Next()。
		// Gin 的 Context/ResponseWriter 不是并发安全的；并发写响应会导致
		// "Headers were already written" 以及在压力下的异常行为。
		//
		// 这里仅通过 request context 注入超时，确保下游（DB/HTTP）可被取消。
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// 如果已超时且还未写响应，兜底返回 504。
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"error": "request timeout"})
		}
	}
}
