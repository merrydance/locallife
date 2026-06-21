package api

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestSanitizeQueryRedactsWechatLoginSecrets(t *testing.T) {
	raw := url.Values{
		"secret":        {"wechat-secret"},
		"code":          {"login-code"},
		"js_code":       {"js-login-code"},
		"session_key":   {"session-key"},
		"access_token":  {"access-token"},
		"refresh_token": {"refresh-token"},
		"safe":          {"visible"},
	}.Encode()

	sanitized := sanitizeQuery(raw)

	require.Contains(t, sanitized, "safe=visible")
	require.Contains(t, sanitized, "secret=%2A%2A%2A")
	require.Contains(t, sanitized, "code=%2A%2A%2A")
	require.Contains(t, sanitized, "js_code=%2A%2A%2A")
	require.Contains(t, sanitized, "session_key=%2A%2A%2A")
	require.NotContains(t, sanitized, "wechat-secret")
	require.NotContains(t, sanitized, "login-code")
	require.NotContains(t, sanitized, "js-login-code")
	require.NotContains(t, sanitized, "session-key")
	require.NotContains(t, sanitized, "access-token")
	require.NotContains(t, sanitized, "refresh-token")
}

func TestRequestLogLevelForScannerTraffic(t *testing.T) {
	scannerUA := "Mozilla/5.0 Tencent Security Team"

	require.Equal(t, zerolog.InfoLevel, requestLogLevel("/anything", http.StatusUnauthorized, scannerUA))
	require.Equal(t, zerolog.InfoLevel, requestLogLevel("/anything", http.StatusForbidden, scannerUA))
	require.Equal(t, zerolog.InfoLevel, requestLogLevel("/anything", http.StatusNotFound, scannerUA))
	require.Equal(t, zerolog.InfoLevel, requestLogLevel("/anything", http.StatusTooManyRequests, scannerUA))
	require.Equal(t, zerolog.ErrorLevel, requestLogLevel("/anything", http.StatusInternalServerError, scannerUA))
	require.Equal(t, zerolog.WarnLevel, requestLogLevel("/anything", http.StatusUnauthorized, "LocalLifeWeapp/1.0"))
}
