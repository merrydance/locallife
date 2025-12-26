package api

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP 请求计数器
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTP 请求延迟直方图
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// 活跃请求数
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	// 数据库连接池指标（需要从外部注入）
	dbConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
	)

	dbConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	// WebSocket 连接数
	wsConnectionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "websocket_connections_total",
			Help: "Total number of WebSocket connections",
		},
		[]string{"type"}, // rider, merchant, platform
	)

	// 业务指标
	ordersCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total number of orders created",
		},
	)

	paymentsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_processed_total",
			Help: "Total number of payments processed",
		},
		[]string{"status"}, // success, failed
	)

	alertsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_sent_total",
			Help: "Total number of alerts sent",
		},
		[]string{"type", "level"},
	)
)

// PrometheusMiddleware 记录 HTTP 请求指标
func PrometheusMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 跳过 /metrics 和 /health 端点
		path := ctx.FullPath()
		if path == "/metrics" || path == "/health" || path == "/ready" {
			ctx.Next()
			return
		}

		// 如果路径为空（404），使用实际路径
		if path == "" {
			path = ctx.Request.URL.Path
		}

		httpRequestsInFlight.Inc()
		start := time.Now()

		ctx.Next()

		httpRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ctx.Writer.Status())

		httpRequestsTotal.WithLabelValues(ctx.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(ctx.Request.Method, path).Observe(duration)
	}
}

// MetricsHandler 返回 Prometheus 指标处理器
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(ctx *gin.Context) {
		h.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

// PrometheusHandler 返回 Prometheus 指标处理器（别名）
func PrometheusHandler() gin.HandlerFunc {
	return MetricsHandler()
}

// UpdateDBMetrics 更新数据库连接池指标（应该定期调用）
func UpdateDBMetrics(active, idle int) {
	dbConnectionsActive.Set(float64(active))
	dbConnectionsIdle.Set(float64(idle))
}

// UpdateWSMetrics 更新 WebSocket 连接数指标
func UpdateWSMetrics(riders, merchants, platforms int) {
	wsConnectionsTotal.WithLabelValues("rider").Set(float64(riders))
	wsConnectionsTotal.WithLabelValues("merchant").Set(float64(merchants))
	wsConnectionsTotal.WithLabelValues("platform").Set(float64(platforms))
}

// RecordOrderCreated 记录订单创建
func RecordOrderCreated() {
	ordersCreatedTotal.Inc()
}

// RecordPaymentProcessed 记录支付处理
func RecordPaymentProcessed(success bool) {
	status := "success"
	if !success {
		status = "failed"
	}
	paymentsProcessedTotal.WithLabelValues(status).Inc()
}

// RecordAlertSent 记录告警发送
func RecordAlertSent(alertType, level string) {
	alertsSentTotal.WithLabelValues(alertType, level).Inc()
}
