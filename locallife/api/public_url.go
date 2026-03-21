package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// externalBaseURL returns the base URL used to build externally-accessible
// resource URLs (e.g. signed upload URLs).
//
// Priority:
//  1. config.ExternalBaseURL — operator-configured, authoritative (production must set this)
//  2. X-Forwarded-Proto + X-Forwarded-Host — set by a trusted reverse proxy
//  3. The Host header from the incoming request
//
// The Origin request header is intentionally excluded: it is fully controlled
// by the client and must never be used to construct server-side URLs
// (open-redirect / SSRF risk).
func (server *Server) externalBaseURL(ctx *gin.Context) string {
	// 1. Operator-configured authoritative base URL (highest priority).
	if server.config.ExternalBaseURL != "" {
		return strings.TrimRight(server.config.ExternalBaseURL, "/")
	}

	// 2. Scheme from X-Forwarded-Proto (set by a trusted reverse proxy).
	scheme := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if ctx.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	// 3. Host from X-Forwarded-Host, then fall back to the Host header.
	//    Never use Origin — it is client-controlled.
	host := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(ctx.Request.Host)
	}
	if host == "" {
		host = "localhost"
	}

	// Force HTTPS for non-local hosts regardless of what the proxy reported.
	if !strings.HasPrefix(host, "localhost") && !strings.HasPrefix(host, "127.0.0.1") {
		scheme = "https"
	}

	return scheme + "://" + host
}
