package baofu

import (
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
