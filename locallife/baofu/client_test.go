package baofu

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublicEnvelopeTimestampUsesBaofooLocalTime(t *testing.T) {
	now := time.Date(2026, 5, 4, 13, 18, 38, 0, time.UTC)

	require.Equal(t, "20260504211838", PublicEnvelopeTimestamp(now))
}

func TestProviderErrorKeepsUpstreamMessageOutOfFrontendGuidance(t *testing.T) {
	err := providerResponseError("unified_order", 200, "BF_UNKNOWN_NEW", "上游原始敏感错误", errors.New("baofu public business response failed"))

	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "BF_UNKNOWN_NEW", providerErr.UpstreamCode)
	require.Equal(t, "上游原始敏感错误", providerErr.UpstreamMessage)
	require.Equal(t, "支付通道异常，请联系平台处理", providerErr.Frontend.Message)
	require.Equal(t, "contact_platform", providerErr.Frontend.Action)
	require.False(t, providerErr.Frontend.Retryable)
	require.NotContains(t, err.Error(), "上游原始")
	require.NotContains(t, providerErr.Frontend.Message, "上游原始")
}

func TestProviderRequestErrorUsesSafeCodeWhenNoUpstreamCodeExists(t *testing.T) {
	err := providerRequestError("T-1001-013-01", 0, "", errors.New("dial tcp 203.0.113.1:443: i/o timeout"))

	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "REQUEST_FAILED", providerErr.UpstreamCode)
	require.Contains(t, err.Error(), "code=REQUEST_FAILED")
	require.NotContains(t, err.Error(), "203.0.113.1")
	require.Equal(t, "支付通道异常，请联系平台处理", providerErr.Frontend.Message)
}

func TestBusinessFailureDetectorsFailClosedForMissingSuccessIndicators(t *testing.T) {
	accountCode, accountMessage, accountFailed := accountBusinessFailure(json.RawMessage(`{"errorCode":"BF0005","errorMsg":"上游账户处理中"}`))
	require.True(t, accountFailed)
	require.Equal(t, "BF0005", accountCode)
	require.Equal(t, "上游账户处理中", accountMessage)

	publicCode, publicMessage, publicFailed := publicBusinessFailure(json.RawMessage(`{"errCode":"MERCHANT_NOT_REPORT","errMsg":"上游报备缺失"}`))
	require.True(t, publicFailed)
	require.Equal(t, "MERCHANT_NOT_REPORT", publicCode)
	require.Equal(t, "上游报备缺失", publicMessage)

	accountCode, accountMessage, accountFailed = accountBusinessFailure(json.RawMessage(`{"contractNo":"CM202605040001"}`))
	require.True(t, accountFailed)
	require.Equal(t, "MISSING_RET_CODE", accountCode)
	require.Empty(t, accountMessage)

	publicCode, publicMessage, publicFailed = publicBusinessFailure(json.RawMessage(`{"outTradeNo":"BF202605040001"}`))
	require.True(t, publicFailed)
	require.Equal(t, "MISSING_RESULT_CODE", publicCode)
	require.Empty(t, publicMessage)
}

func TestAccountBusinessFailureAcceptsNumericRetCode(t *testing.T) {
	code, message, failed := accountBusinessFailure(json.RawMessage(`{"retCode":0,"errorCode":"BF00061","errorMsg":"上游四要素失败"}`))
	require.True(t, failed)
	require.Equal(t, "BF00061", code)
	require.Equal(t, "上游四要素失败", message)

	code, message, failed = accountBusinessFailure(json.RawMessage(`{"retCode":1,"result":[{"state":2,"transSerialNo":"OPEN202605050001"}]}`))
	require.False(t, failed)
	require.Empty(t, code)
	require.Empty(t, message)
}

func TestPublicBusinessFailureUsesUnknownNonSuccessResultCode(t *testing.T) {
	code, message, failed := publicBusinessFailure(json.RawMessage(`{"resultCode":"PENDING_REVIEW","errMsg":"上游未知处理中"}`))

	require.True(t, failed)
	require.Equal(t, "PENDING_REVIEW", code)
	require.Equal(t, "上游未知处理中", message)

	err := providerResponseError("merchant_report", 200, code, message, errors.New("baofu public business response failed"))
	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "PENDING_REVIEW", providerErr.UpstreamCode)
	require.Equal(t, "上游未知处理中", providerErr.UpstreamMessage)
	require.Equal(t, "BAOFU_MANUAL_REVIEW", providerErr.Frontend.Code)
	require.Equal(t, "支付通道异常，请联系平台处理", providerErr.Frontend.Message)
	require.NotContains(t, providerErr.Frontend.Message, "上游未知")
}

func TestPublicBusinessFailureFailsForSuccessResultWithFailureErrCode(t *testing.T) {
	code, message, failed := publicBusinessFailure(json.RawMessage(`{"resultCode":"SUCCESS","errCode":"ORDER_NOT_EXIST","errMsg":"上游订单不存在"}`))

	require.True(t, failed)
	require.Equal(t, "ORDER_NOT_EXIST", code)
	require.Equal(t, "上游订单不存在", message)

	code, message, failed = publicBusinessFailure(json.RawMessage(`{"resultCode":"SUCCESS","errCode":"SUCCESS","errMsg":"OK"}`))
	require.False(t, failed)
	require.Empty(t, code)
	require.Empty(t, message)
}
