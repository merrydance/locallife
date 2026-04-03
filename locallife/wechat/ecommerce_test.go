package wechat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type ecommerceRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn ecommerceRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCreateEcommerceApplyment_SetsWechatpaySerialHeader(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	_, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "test_mch_id",
			AppID:                 "test_app_id",
			SerialNumber:          "test_serial",
			APIV3Key:              "test_api_v3_key_32bytes_long__",
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: ecommerceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "PUB_KEY_ID_0123456789", req.Header.Get("Wechatpay-Serial"))
			require.NotEmpty(t, req.Header.Get("Authorization"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			require.Equal(t, "4", body["organization_type"])
			require.Equal(t, false, body["finance_institution"])
			require.NotContains(t, body, "need_account_info")

			accountInfo, ok := body["account_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "75", accountInfo["bank_account_type"])

			contactInfo, ok := body["contact_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "65", contactInfo["contact_type"])

			idCardInfo, ok := body["id_card_info"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "2020-01-01", idCardInfo["id_card_valid_time_begin"])
			require.Equal(t, "长期", idCardInfo["id_card_valid_time"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"applyment_id":123456789}`)),
			}, nil
		}),
	}

	resp, err := client.CreateEcommerceApplyment(context.Background(), &EcommerceApplymentRequest{
		OutRequestNo:       "applyment-test-001",
		OrganizationType:   "4",
		FinanceInstitution: false,
		MerchantShortname:  "测试运营商",
		IDCardInfo:         &ApplymentIDCardInfo{IDCardCopy: "copy_media_id", IDCardNational: "national_media_id", IDCardName: "encrypted_name", IDCardNumber: "encrypted_id_no", IDCardValidTimeBegin: "2020-01-01", IDCardValidTime: "长期"},
		AccountInfo:        &ApplymentBankAccountInfo{BankAccountType: "ACCOUNT_TYPE_PRIVATE", AccountBank: "工商银行", AccountName: "encrypted_account_name", BankAddressCode: "440300", AccountNumber: "encrypted_account_no"},
		ContactInfo:        &ApplymentContactInfo{ContactType: "LEGAL", ContactName: "encrypted_contact_name", MobilePhone: "encrypted_mobile", ContactEmail: "encrypted_email@example.com"},
		SalesSceneInfo:     &ApplymentSalesSceneInfo{StoreName: "测试门店", StoreURL: "https://example.com/store"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(123456789), resp.ApplymentID)
}
