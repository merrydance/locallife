package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const clientLogSharedKeyHeader = "X-Client-Log-Key"

func isScannerUserAgent(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return false
	}
	return strings.Contains(ua, "tencent security team") || strings.Contains(ua, "scanner")
}

// reportClientErrorLog 接收前端错误日志上报（匿名、尽力而为）
// @Summary 上报前端错误日志
// @Description 小程序/网页前端将错误日志异步上报到后端，用于排查问题。该接口不影响主业务流程。
// @Tags 监控
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "前端错误日志"
// @Success 200 {object} MessageResponse "ok"
// @Failure 400 {object} ErrorResponse "请求体非法"
// @Router /v1/logs/error [post]
func (server *Server) reportClientErrorLog(ctx *gin.Context) {
	userAgent := ctx.GetHeader("User-Agent")

	if isScannerUserAgent(userAgent) {
		log.Info().
			Str("request_id", GetRequestID(ctx)).
			Str("client_ip", ctx.ClientIP()).
			Str("user_agent", userAgent).
			Msg("drop scanner traffic for frontend error log")
		ctx.JSON(http.StatusOK, MessageResponse{Message: "ok"})
		return
	}

	sharedKey := strings.TrimSpace(server.config.ClientLogSharedKey)
	if sharedKey != "" {
		providedKey := strings.TrimSpace(ctx.GetHeader(clientLogSharedKeyHeader))
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(sharedKey)) != 1 {
			log.Info().
				Str("request_id", GetRequestID(ctx)).
				Str("client_ip", ctx.ClientIP()).
				Str("user_agent", userAgent).
				Msg("drop untrusted frontend error log source")
			ctx.JSON(http.StatusOK, MessageResponse{Message: "ok"})
			return
		}
	}

	var payload map[string]interface{}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	log.Warn().
		Str("request_id", GetRequestID(ctx)).
		Str("client_ip", ctx.ClientIP()).
		Str("user_agent", userAgent).
		Str("path", ctx.Request.URL.Path).
		Interface("payload", payload).
		Msg("frontend error log received")

	ctx.JSON(http.StatusOK, MessageResponse{Message: "ok"})
}
