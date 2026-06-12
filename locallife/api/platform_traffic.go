package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type trafficSummaryQuery struct {
	WindowSeconds int `form:"window_seconds"`
	RouteLimit    int `form:"route_limit"`
}

// getPlatformTrafficSummary returns recent HTTP traffic statistics for the platform console.
// @Summary 获取平台流量汇总
// @Description 返回最近时间窗口内的 HTTP 请求、响应字节与路由排行快照
// @Tags Platform
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param window_seconds query int false "窗口秒数，默认 300"
// @Param route_limit query int false "路由条数，默认 20"
// @Success 200 {object} trafficSummaryResponse
// @Router /v1/platform/stats/traffic/summary [get]
func (server *Server) getPlatformTrafficSummary(ctx *gin.Context) {
	var req trafficSummaryQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	windowSeconds := req.WindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = defaultTrafficWindowSeconds
	}
	routeLimit := req.RouteLimit
	if routeLimit <= 0 {
		routeLimit = defaultTrafficRouteLimit
	}

	summary := globalTrafficRecorder.summary(windowSeconds, routeLimit)

	ctx.JSON(http.StatusOK, summary)
}
