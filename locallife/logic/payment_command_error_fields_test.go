package logic

import (
	"testing"

	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	"github.com/stretchr/testify/require"
)

func TestPartnerPaymentCommandErrorFieldsIncludeOrdinaryProviderAction(t *testing.T) {
	err := &ordinaryserviceprovider.ProviderError{
		ProviderCode: "REQUEST_BLOCKED",
		Frontend: ordinaryserviceprovider.FrontendGuidance{
			Message: "特约商户能力被微信支付限制",
			Action:  "请在平台财务-普通服务商商户管控诊断中查询受限能力和解脱路径",
		},
	}

	code, message := partnerPaymentCommandErrorFields(err)

	require.NotNil(t, code)
	require.Equal(t, "REQUEST_BLOCKED", *code)
	require.NotNil(t, message)
	require.Contains(t, *message, "特约商户能力被微信支付限制")
	require.Contains(t, *message, "平台财务-普通服务商商户管控诊断")
}
