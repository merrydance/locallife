package baofu

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validBaofuConfigForTest() Config {
	return Config{
		Environment:        BaofuEnvironmentSandbox,
		CollectMerchantID:  "102004465",
		CollectTerminalID:  "200005200",
		PayoutMerchantID:   "102004466",
		PayoutTerminalID:   "200005201",
		AppID:              "local-life-miniapp",
		PrivateKeyPEM:      "test-private-key",
		BaofuPublicKeyPEM:  "test-public-key",
		SignSerialNo:       "1",
		EncryptionSerialNo: "1",
		NotifyBaseURL:      "https://pay.example.com/callbacks/baofu",
		Timeout:            10 * time.Second,
	}
}

func TestConfigValidateDoesNotRequireAESKey(t *testing.T) {
	cfg := validBaofuConfigForTest()

	require.NoError(t, cfg.Validate())
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

func TestConfigValidateRejectsPublicEnvelopeSerialsLongerThanOfficialLimit(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.SignSerialNo = "823078EFB7E1BBDC"

	require.EqualError(t, cfg.Validate(), "baofu sign serial no must be at most 10 characters")

	cfg = validBaofuConfigForTest()
	cfg.EncryptionSerialNo = "B9F3E90C370A7A0F"

	require.EqualError(t, cfg.Validate(), "baofu encryption serial no must be at most 10 characters")
}

func TestConfigNormalizedDefaultsTimeoutAndOfficialEndpoints(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.AccountGatewayBaseURL = "  "
	cfg.AggregatePayBaseURL = "  "
	cfg.AggregatePayBackupBaseURL = "  "
	cfg.MerchantReportBaseURL = "  "
	cfg.Timeout = 0

	normalized := cfg.Normalized()

	require.Equal(t, SandboxAccountGatewayBaseURL, normalized.AccountGatewayBaseURL)
	require.Equal(t, SandboxAggregatePayBaseURL, normalized.AggregatePayBaseURL)
	require.Equal(t, SandboxMerchantReportBaseURL, normalized.MerchantReportBaseURL)
	require.Equal(t, 30*time.Second, normalized.Timeout)
}

func TestConfigValidateRejectsPlaceholderEndpoint(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.AggregatePayBaseURL = "https://api.baofoo.com"

	require.EqualError(t, cfg.Validate(), "baofu aggregate pay base url must be an official endpoint")
}

func TestConfigValidateRequiresOfficialEndpointProfiles(t *testing.T) {
	cfg := validBaofuConfigForTest()
	cfg.AccountGatewayBaseURL = "https://test-api.example.com/union-gw/api"

	require.EqualError(t, cfg.Validate(), "baofu account gateway base url must be an official endpoint")

	cfg = validBaofuConfigForTest()
	cfg.MerchantReportBaseURL = "https://test-api.example.com/mch-service/api"

	require.EqualError(t, cfg.Validate(), "baofu merchant report base url must be an official endpoint")
}
