package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/token"
	"golang.org/x/time/rate"
)

// RateLimiterConfig 速率限制配置
type RateLimiterConfig struct {
	// 基于 IP 的限流
	IPRateLimit  rate.Limit // 每秒允许的请求数
	IPBurstLimit int        // 突发请求数

	// 基于用户的限流（已认证用户）
	UserRateLimit  rate.Limit
	UserBurstLimit int

	// 清理间隔（清理过期的限流器）
	CleanupInterval time.Duration
}

// DefaultRateLimiterConfig 默认配置
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		IPRateLimit:     10,              // 每秒 10 个请求
		IPBurstLimit:    20,              // 允许突发 20 个
		UserRateLimit:   20,              // 认证用户每秒 20 个
		UserBurstLimit:  50,              // 允许突发 50 个
		CleanupInterval: 10 * time.Minute, // 每 10 分钟清理一次
	}
}

// visitor 存储每个访问者的限流器和最后访问时间
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter 速率限制器
type RateLimiter struct {
	config   RateLimiterConfig
	visitors map[string]*visitor
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewRateLimiter 创建新的速率限制器
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:   config,
		visitors: make(map[string]*visitor),
		stopCh:   make(chan struct{}),
	}

	// 启动后台清理协程
	go rl.cleanupVisitors()

	return rl
}

// getVisitor 获取或创建访问者的限流器
func (rl *RateLimiter) getVisitor(key string, rateLimit rate.Limit, burst int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(rateLimit, burst)
		rl.visitors[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors 定期清理过期的访问者
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for key, v := range rl.visitors {
				if time.Since(v.lastSeen) > rl.config.CleanupInterval*3 {
					delete(rl.visitors, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop 停止清理协程
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Middleware 返回 Gin 中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 获取客户端 IP
		clientIP := ctx.ClientIP()

		// 检查是否有认证用户
		var key string
		var rateLimit rate.Limit
		var burst int

		if payload, exists := ctx.Get(authorizationPayloadKey); exists {
			// 已认证用户：使用 user_id 作为 key，享受更高的限额
			userPayload := payload.(*token.Payload)
			key = "user:" + strconv.FormatInt(userPayload.UserID, 10)
			rateLimit = rl.config.UserRateLimit
			burst = rl.config.UserBurstLimit
		} else {
			// 未认证用户：使用 IP 作为 key
			key = "ip:" + clientIP
			rateLimit = rl.config.IPRateLimit
			burst = rl.config.IPBurstLimit
		}

		limiter := rl.getVisitor(key, rateLimit, burst)

		if !limiter.Allow() {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "rate limit exceeded, please slow down",
			})
			return
		}

		ctx.Next()
	}
}

// SensitiveAPIMiddleware 敏感接口限流（更严格）
// 用于短信验证码、支付等接口
func (rl *RateLimiter) SensitiveAPIMiddleware(ratePerMinute int) gin.HandlerFunc {
	// 敏感接口使用独立的限流器映射
	sensitiveVisitors := make(map[string]*visitor)
	var mu sync.RWMutex

	return func(ctx *gin.Context) {
		clientIP := ctx.ClientIP()
		key := "sensitive:" + clientIP

		mu.Lock()
		v, exists := sensitiveVisitors[key]
		if !exists {
			// 敏感接口：每分钟 N 次
			limiter := rate.NewLimiter(rate.Limit(float64(ratePerMinute)/60.0), ratePerMinute)
			sensitiveVisitors[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
			v = sensitiveVisitors[key]
		}
		v.lastSeen = time.Now()
		mu.Unlock()

		if !v.limiter.Allow() {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "too many requests for this operation, please try again later",
			})
			return
		}

		ctx.Next()
	}
}
