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

func TestIsProviderBusinessResponseErrorDistinguishesBusinessAndContractFailures(t *testing.T) {
	require.True(t, IsProviderBusinessResponseError(NewProviderBusinessError("T-1001-013-03", "BF00064", "account not found")))
	require.True(t, IsProviderBusinessResponseError(providerResponseError("T-1001-013-03", 200, "BF00064", "account not found", errProviderAccountBusinessResponse)))
	require.False(t, IsProviderBusinessResponseError(NewProviderContractError("T-1001-013-03", errors.New("contractNo is required"))))
	require.False(t, IsProviderBusinessResponseError(providerRequestError("T-1001-013-03", 0, "", errors.New("dial tcp timeout"))))
}

func TestBusinessFailureDetectorsFailClosedForMissingSuccessIndicators(t *testing.T) {
	accountFailure := accountBusinessFailure(json.RawMessage(`{"errorCode":"BF0005","errorMsg":"上游账户处理中"}`))
	require.True(t, accountFailure.Failed)
	require.Equal(t, "BF0005", accountFailure.Code)
	require.Equal(t, "上游账户处理中", accountFailure.Message)
	require.Equal(t, "body.errorCode", accountFailure.SourcePath)

	accountFailure = accountBusinessFailure(json.RawMessage(`{"errorMsg":"上游账户失败但未返回错误码"}`))
	require.True(t, accountFailure.Failed)
	require.Equal(t, "MISSING_RET_CODE", accountFailure.Code)
	require.Equal(t, "上游账户失败但未返回错误码", accountFailure.Message)
	require.Equal(t, "body.retCode", accountFailure.SourcePath)

	accountFailure = accountBusinessFailure(json.RawMessage(`{"contractNo":"CM202605040001"}`))
	require.True(t, accountFailure.Failed)
	require.Equal(t, "MISSING_RET_CODE", accountFailure.Code)
	require.Empty(t, accountFailure.Message)
	require.Equal(t, "body.retCode", accountFailure.SourcePath)
}

func TestAccountBusinessFailureAcceptsNumericRetCode(t *testing.T) {
	failure := accountBusinessFailure(json.RawMessage(`{"retCode":0,"errorCode":"BF00061","errorMsg":"上游四要素失败"}`))
	require.True(t, failure.Failed)
	require.Equal(t, "BF00061", failure.Code)
	require.Equal(t, "上游四要素失败", failure.Message)
	require.Equal(t, "body.errorCode", failure.SourcePath)

	failure = accountBusinessFailure(json.RawMessage(`{"retCode":1,"result":[{"state":2,"transSerialNo":"OPEN202605050001"}]}`))
	require.False(t, failure.Failed)
	require.Empty(t, failure.Code)
	require.Empty(t, failure.Message)
}

func TestAccountBusinessFailureTreatsAcceptedResultFailureAsBusinessResult(t *testing.T) {
	failure := accountBusinessFailure(json.RawMessage(`{"retCode":1,"result":[{"state":0,"errorCode":"BF00061","errorMsg":"企业法人四要素验证失败"}]}`))

	require.False(t, failure.Failed)
	require.Empty(t, failure.Code)
	require.Empty(t, failure.Message)
}
