package wechat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuerySubMerchantLimitations(t *testing.T) {
	merchantPrivateKey, _ := generateTestKeyPair(t)
	platformPrivateKey, platformPublicKey := generateTestKeyPair(t)

	tempDir := t.TempDir()
	privateKeyPath := createTestPrivateKeyFile(t, tempDir, merchantPrivateKey)
	publicKeyPath := createTestPublicKeyFile(t, tempDir, platformPublicKey)

	client, err := NewEcommerceClient(EcommerceClientConfig{
		PaymentClientConfig: PaymentClientConfig{
			MchID:                 "service-mchid-001",
			AppID:                 "service-appid-001",
			SerialNumber:          "test_serial",
			APIV3Key:              testAPIV3Key(),
			PrivateKeyPath:        privateKeyPath,
			PlatformPublicKeyPath: publicKeyPath,
			PlatformPublicKeyID:   "PUB_KEY_ID_0123456789",
			NotifyURL:             "https://example.com/notify",
		},
		SpMchID: "service-mchid-001",
		SpAppID: "service-appid-001",
	})
	require.NoError(t, err)

	client.httpClient = &http.Client{
		Transport: signedEcommerceTransport(t, platformPrivateKey, "PUB_KEY_ID_0123456789", func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "/v3/mch-operation-manage/merchant-limitations/sub-mchid/1900000109", req.URL.Path)
			require.Equal(t, "application/json", req.Header.Get("Accept"))
			require.NotEmpty(t, req.Header.Get("Authorization"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"mchid":"1900000109","limited_functions":["NO_TRANSACTION_AND_RECHARGE"],"other_limited_functions":"关闭相册扫码支付,关闭长按识别支付","recovery_specifications":[{"limitation_case_id":"A20250819155047774441874","limitation_reason_type":"LICENSE_ABNORMAL","limitation_reason":"入驻后180天无账户动账","limitation_reason_describe":"当前商户号入驻后长时间无账户动账，请重新确认开户意愿并核实身份","relate_limitations":["NO_TRANSACTION_AND_RECHARGE"],"other_relate_limitations":"关闭相册扫码支付,关闭长按识别支付","recover_way":"MODIFY_SUBJECT_INFORMATION","recover_way_param":"100200300112233","recover_help_url":"https://kf.qq.com","limitation_action_type":"LIMIT_ACTION_TYPE_IMMEDIATE_CONTROL","limitation_start_date":"2025-06-08T10:34:56+08:00","limitation_date":"2025-06-08T10:34:56+08:00"}]}`)),
			}, nil
		}),
	}

	resp, err := client.QuerySubMerchantLimitations(context.Background(), "1900000109")
	require.NoError(t, err)
	require.Equal(t, "1900000109", resp.MchID)
	require.Equal(t, []string{"NO_TRANSACTION_AND_RECHARGE"}, resp.LimitedFunctions)
	require.Equal(t, "关闭相册扫码支付,关闭长按识别支付", resp.OtherLimitedFunctions)
	require.Len(t, resp.RecoverySpecifications, 1)
	require.Equal(t, "LICENSE_ABNORMAL", resp.RecoverySpecifications[0].LimitationReasonType)
	require.Equal(t, "MODIFY_SUBJECT_INFORMATION", resp.RecoverySpecifications[0].RecoverWay)
}

func TestQuerySubMerchantLimitationsRequiresSubMchID(t *testing.T) {
	client := &EcommerceClient{}
	resp, err := client.QuerySubMerchantLimitations(context.Background(), " ")
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub_mchid is required")
}
