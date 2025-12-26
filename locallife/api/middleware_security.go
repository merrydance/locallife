package api

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware 安全响应头中间件
// 添加常见的安全 HTTP 头，防止常见攻击
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 防止点击劫持
		// DENY: 完全禁止被嵌入 iframe
		// SAMEORIGIN: 只允许同源嵌入
		ctx.Header("X-Frame-Options", "DENY")

		// 防止 MIME 类型嗅探攻击
		ctx.Header("X-Content-Type-Options", "nosniff")

		// 启用 XSS 过滤（现代浏览器已内置，但作为兜底）
		ctx.Header("X-XSS-Protection", "1; mode=block")

		// Referrer 策略：只在同源请求时发送完整 referrer
		ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 内容安全策略（CSP）- API 服务器通常不需要，但可以设置基本策略
		// 禁止内联脚本和外部资源
		ctx.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// 禁止浏览器缓存敏感响应（针对 API 响应）
		ctx.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		ctx.Header("Pragma", "no-cache")
		ctx.Header("Expires", "0")

		// 权限策略：禁用危险的浏览器特性
		ctx.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		ctx.Next()
	}
}

// HSTSMiddleware 强制 HTTPS 中间件
// 注意：只在通过 HTTPS 访问时才应该启用
// 在 Nginx 后面时，通常由 Nginx 设置此头
func HSTSMiddleware(maxAge int) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 检查是否通过 HTTPS（考虑反向代理）
		if ctx.Request.TLS != nil || ctx.GetHeader("X-Forwarded-Proto") == "https" {
			// max-age: 浏览器记住此策略的时间（秒）
			// includeSubDomains: 应用到所有子域名
			// preload: 允许加入浏览器预加载列表
			ctx.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		ctx.Next()
	}
}

// CORSMiddleware 跨域资源共享中间件
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	// 构建允许的源映射，用于快速查找
	originsMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		originsMap[origin] = true
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")

		// 检查是否为允许的源
		if origin != "" && (len(allowedOrigins) == 0 || originsMap[origin] || originsMap["*"]) {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Access-Control-Allow-Credentials", "true")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
			ctx.Header("Access-Control-Expose-Headers", "X-Request-ID")
			ctx.Header("Access-Control-Max-Age", "86400") // 预检请求缓存 24 小时
		}

		// 处理预检请求
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
			return
		}

		ctx.Next()
	}
}
