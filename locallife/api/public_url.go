package api

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

func externalBaseURL(ctx *gin.Context) string {
	scheme := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if ctx.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(ctx.Request.Host)
	}
	if host == "" {
		host = "localhost"
	}

	// If host has no port and original request has one, keep it.
	// This is mainly for local/dev; in production X-Forwarded-Host should already be correct.
	if !strings.Contains(host, ":") {
		if reqHost, reqPort, err := net.SplitHostPort(ctx.Request.Host); err == nil {
			if reqHost != "" && reqPort != "" {
				host = host + ":" + reqPort
			}
		}
	}

	return scheme + "://" + host
}

func normalizeUploadPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}

	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}

	p = strings.TrimPrefix(p, "/")
	if strings.HasPrefix(p, "uploads/") {
		return p
	}

	return "uploads/" + p
}
