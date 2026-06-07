package worker

import (
	"testing"

	"github.com/merrydance/locallife/baofu"
	"github.com/stretchr/testify/require"
)

func TestWorkerPaymentCommandErrorFieldsIncludeBaofuProviderGuidance(t *testing.T) {
	err := &baofu.ProviderError{
		Operation:       "order_refund",
		UpstreamCode:    "REFUND_AMT_EXCEEDS",
		UpstreamMessage: "退款金额超过可退金额",
		Frontend:        baofu.ClassifyBaofuError("REFUND_AMT_EXCEEDS", "退款金额超过可退金额").FrontendGuidance(),
	}

	code, message := workerPaymentCommandErrorFields(err)

	require.NotNil(t, code)
	require.Equal(t, "REFUND_AMT_EXCEEDS", *code)
	require.NotNil(t, message)
	require.Contains(t, *message, "资料信息不完整，请核对后重新提交")
	require.Contains(t, *message, "退款金额超过可退金额")
	require.Contains(t, *message, "check_and_resubmit")
}

func TestWorkerPaymentCommandErrorFieldsPreferBaofuProviderGuidanceOverRawErrorText(t *testing.T) {
	err := &baofu.ProviderError{
		Operation:       "order_refund",
		UpstreamCode:    "REFUND_AMT_EXCEEDS",
		UpstreamMessage: "退款金额超过可退金额",
		Frontend:        baofu.ClassifyBaofuError("REFUND_AMT_EXCEEDS", "退款金额超过可退金额").FrontendGuidance(),
	}

	code, message := workerPaymentCommandErrorFields(err)

	require.NotNil(t, code)
	require.Equal(t, "REFUND_AMT_EXCEEDS", *code)
	require.NotNil(t, message)
	require.Equal(t, "资料信息不完整，请核对后重新提交：退款金额超过可退金额，check_and_resubmit", *message)
}
