package baofu

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validBaofuConfigForTest() Config {
	return Config{
		BaseURL:           "https://test-api.example.com",
		CollectMerchantID: "102004465",
		CollectTerminalID: "200005200",
		PayoutMerchantID:  "102004466",
		PayoutTerminalID:  "200005201",
		AppID:             "local-life-miniapp",
		PrivateKeyPEM:     "test-private-key",
		BaofuPublicKeyPEM: "test-public-key",
		AESKey:            "0123456789abcdef0123456789abcdef",
		NotifyBaseURL:     "https://pay.example.com/callbacks/baofu",
		Timeout:           10 * time.Second,
	}
}

func TestConfigValidateRequiresSeparatedMerchants(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.PayoutMerchantID = cfg.CollectMerchantID

	require.EqualError(t, cfg.Validate(), "baofu collect merchant id and payout merchant id must be different")
}

func TestConfigValidateRequiresCollectMerchant(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.CollectMerchantID = "  "

	require.EqualError(t, cfg.Validate(), "baofu collect merchant id is required")
}

func TestConfigNormalizedDefaultsTimeoutAndBaseURL(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.BaseURL = "  "
	cfg.Timeout = 0

	normalized := cfg.Normalized()

	require.Equal(t, DefaultBaseURL, normalized.BaseURL)
	require.Equal(t, 30*time.Second, normalized.Timeout)
}
