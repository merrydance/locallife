package worker

import (
	"testing"

	"github.com/merrydance/locallife/baofu"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	"github.com/stretchr/testify/require"
)

func TestWorkerPaymentCommandErrorFieldsIncludeOrdinaryProviderAction(t *testing.T) {
	err := &ordinaryserviceprovider.ProviderError{
		ProviderCode: "REQUEST_BLOCKED",
		Frontend: ordinaryserviceprovider.FrontendGuidance{
			Message: "特约商户能力被微信支付限制",
			Action:  "请在平台财务-普通服务商商户管控诊断中查询受限能力和解脱路径",
		},
	}

	code, message := workerPaymentCommandErrorFields(err)

	require.NotNil(t, code)
	require.Equal(t, "REQUEST_BLOCKED", *code)
	require.NotNil(t, message)
	require.Contains(t, *message, "特约商户能力被微信支付限制")
	require.Contains(t, *message, "平台财务-普通服务商商户管控诊断")
}

func TestWorkerPaymentCommandErrorFieldsIncludeBaofuProviderGuidance(t *testing.T) {
	err := &baofu.ProviderError{
		Operation:       "order_refund",
		UpstreamCode:    "REFUND_AMT_EXCEEDS",
		UpstreamMessage: "raw upstream refund amount detail",
		Frontend:        baofu.ClassifyBaofuError("REFUND_AMT_EXCEEDS", "raw upstream refund amount detail").FrontendGuidance(),
	}

	code, message := workerPaymentCommandErrorFields(err)

	require.NotNil(t, code)
	require.Equal(t, "REFUND_AMT_EXCEEDS", *code)
	require.NotNil(t, message)
	require.Contains(t, *message, "资料信息不完整，请核对后重新提交")
	require.Contains(t, *message, "check_and_resubmit")
	require.NotContains(t, *message, "raw upstream")
}

func TestWorkerPaymentCommandErrorFieldsPreferBaofuProviderGuidanceOverRawErrorText(t *testing.T) {
	err := &baofu.ProviderError{
		Operation:       "order_refund",
		UpstreamCode:    "REFUND_AMT_EXCEEDS",
		UpstreamMessage: "raw upstream refund amount detail",
		Frontend:        baofu.ClassifyBaofuError("REFUND_AMT_EXCEEDS", "raw upstream refund amount detail").FrontendGuidance(),
	}

	code, message := workerPaymentCommandErrorFields(err)

	require.NotNil(t, code)
	require.Equal(t, "REFUND_AMT_EXCEEDS", *code)
	require.NotNil(t, message)
	require.Equal(t, "资料信息不完整，请核对后重新提交，check_and_resubmit", *message)
	require.NotContains(t, *message, "raw upstream")
}
