package baofu

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestRequestMetadataRedactsSensitiveValues(t *testing.T) {
	metadata := RequestMetadata{
		Capability:    "baofu_account",
		OutRequestNo:  "OPEN123",
		HTTPStatus:    200,
		UpstreamCode:  "SUCCESS",
		IDCardNo:      "130102199001011234",
		BankCardNo:    "6222020202020202020",
		Mobile:        "13800138000",
		PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----secret",
	}

	fields := metadata.SafeLogFields()

	require.Equal(t, "baofu", fields["provider"])
	require.Equal(t, "baofu_account", fields["capability"])
	require.Equal(t, "OPEN123", fields["out_request_no"])
	require.Equal(t, "SUCCESS", fields["upstream_code"])
	require.NotContains(t, fields, "id_card_no")
	require.NotContains(t, fields, "bank_card_no")
	require.NotContains(t, fields, "mobile")
	require.NotContains(t, fields, "private_key_pem")
	require.NotContains(t, fields, "aes_key")
}

func TestLogProviderErrorAcceptsNilError(t *testing.T) {
	LogProviderError(zerolog.Nop(), nil, RequestMetadata{Capability: "baofu_payment"})
}

func TestLogProviderErrorRecordsCleanUpstreamMessage(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	err := &ProviderError{
		Operation:       "T-1001-013-01",
		Capability:      "baofu_account",
		StatusCode:      200,
		RequestID:       "REQ-BAOFU-1",
		UpstreamCode:    "BF0020",
		UpstreamMessage: "统一社会信用代码格式不正确",
		Frontend:        ClassifyBaofuError("BF0020", "统一社会信用代码格式不正确").FrontendGuidance(),
	}

	LogProviderError(logger, err, RequestMetadata{Capability: "baofu_account", OutRequestNo: "OPEN123"})

	require.Contains(t, logs.String(), `"upstream_message_sanitized":"统一社会信用代码格式不正确"`)
	require.Contains(t, logs.String(), `"upstream_code":"BF0020"`)
	require.Contains(t, logs.String(), `"out_request_no":"OPEN123"`)
}

func TestLogProviderErrorRecordsSanitizedUpstreamMessage(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	err := &ProviderError{
		Operation:       "T-1001-013-01",
		Capability:      "baofu_account",
		StatusCode:      200,
		UpstreamCode:    "ID_CARD_CHECK_FAILED",
		UpstreamMessage: "身份证 110101199001010011 银行卡 6222020202020202 手机 13800138000 loginNo=LLBFOO0000000999",
		Frontend:        ClassifyBaofuError("ID_CARD_CHECK_FAILED", "身份证 110101199001010011 银行卡 6222020202020202 手机 13800138000 loginNo=LLBFOO0000000999").FrontendGuidance(),
	}

	LogProviderError(logger, err, RequestMetadata{Capability: "baofu_account"})

	require.Contains(t, logs.String(), `"upstream_message_sanitized"`)
	require.NotContains(t, logs.String(), "110101199001010011")
	require.NotContains(t, logs.String(), "6222020202020202")
	require.NotContains(t, logs.String(), "13800138000")
	require.NotContains(t, logs.String(), "LLBFOO0000000999")
	require.Contains(t, logs.String(), `110101********0011`)
	require.Contains(t, logs.String(), `************0202`)
	require.Contains(t, logs.String(), `138****8000`)
	require.Contains(t, logs.String(), `loginNo=<redacted>`)
}
