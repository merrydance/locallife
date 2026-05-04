package baofu

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyBaofuErrorCodeForFrontendSemantics(t *testing.T) {
	cases := []struct {
		code         string
		wantCategory BaofuErrorCategory
		wantPublic   string
	}{
		{"PARAM_ERROR", BaofuErrorCategoryUserActionRequired, "资料信息不完整，请核对后重新提交"},
		{"MERCHANT_NOT_REPORTED", BaofuErrorCategoryPlatformConfiguration, "微信支付通道待开通，请联系平台处理"},
		{"SYSTEM_BUSY", BaofuErrorCategoryRetryable, "支付通道处理中，请稍后重试"},
		{"UNKNOWN", BaofuErrorCategoryManualReview, "支付通道异常，请联系平台处理"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got := ClassifyBaofuError(tc.code, "raw upstream message")
			require.Equal(t, tc.wantCategory, got.Category)
			require.Equal(t, tc.wantPublic, got.PublicMessage)
			require.NotContains(t, got.PublicMessage, "raw upstream")
		})
	}
}
