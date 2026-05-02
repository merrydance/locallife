package ordinaryserviceprovider

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

func TestMapSDKAPIErrorPreservesDiagnosticsAndFrontendGuidance(t *testing.T) {
	sdkErr := &core.APIError{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"Request-Id": []string{"req-ordinary-123"},
		},
		Code:    "NO_AUTH",
		Message: "sub merchant is not authorized",
	}

	err := mapSDKAPIError("create ordinary payment", sdkErr)

	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "create ordinary payment", providerErr.Operation)
	require.Equal(t, http.StatusForbidden, providerErr.StatusCode)
	require.Equal(t, "req-ordinary-123", providerErr.RequestID)
	require.Equal(t, "NO_AUTH", providerErr.ProviderCode)
	require.Equal(t, ErrorCategoryAuthConfig, providerErr.Category)
	require.Equal(t, "WECHAT_AUTH_CONFIG_REQUIRED", providerErr.Frontend.Code)
	require.NotEmpty(t, providerErr.Frontend.Message)
	require.NotEmpty(t, providerErr.Frontend.Action)
}

func TestLogProviderErrorWritesSemanticDiagnostics(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output)
	providerErr := &ProviderError{
		Operation:    "query merchant limitation",
		StatusCode:   http.StatusTooManyRequests,
		RequestID:    "req-log-001",
		ProviderCode: "FREQUENCY_LIMITED",
		Category:     ErrorCategoryRetryableProvider,
		Frontend: FrontendGuidance{
			Code:      "WECHAT_PROVIDER_RETRYABLE",
			Message:   "微信支付服务暂时不可用，请稍后重试",
			Action:    "稍后重试；如果持续失败请联系平台处理",
			Retryable: true,
		},
	}

	LogProviderError(logger, providerErr, ErrorLogContext{SubMchID: "1900000110"})

	logLine := output.String()
	require.Contains(t, logLine, "ordinary_service_provider")
	require.Contains(t, logLine, "query merchant limitation")
	require.Contains(t, logLine, "req-log-001")
	require.Contains(t, logLine, "FREQUENCY_LIMITED")
	require.Contains(t, logLine, "WECHAT_PROVIDER_RETRYABLE")
	require.NotContains(t, logLine, "plaintext")
}

func TestMapNonSDKErrorKeepsOperationAndRetryGuidance(t *testing.T) {
	err := mapSDKAPIError("submit applyment", errors.New("connection reset"))

	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "submit applyment", providerErr.Operation)
	require.Equal(t, ErrorCategoryRetryableProvider, providerErr.Category)
	require.Equal(t, "WECHAT_PROVIDER_RETRYABLE", providerErr.Frontend.Code)
}
