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
		{"MERCHANT_NOT_REPORTED", BaofuErrorCategoryPlatformConfiguration, "商户微信支付通道待开通，请联系平台处理"},
		{"PAY_CHANNEL_NOT_SUPPORT", BaofuErrorCategoryPlatformConfiguration, "商户微信支付通道待开通，请联系平台处理"},
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

func TestClassifyBaofuOfficialErrorTables(t *testing.T) {
	cases := []struct {
		name          string
		code          string
		wantCategory  BaofuErrorCategory
		wantMessage   string
		wantAction    string
		wantRetryable bool
	}{
		{name: "account parameter", code: "BF0001", wantCategory: BaofuErrorCategoryUserActionRequired, wantMessage: "资料信息不完整，请核对后重新提交", wantAction: "check_and_resubmit"},
		{name: "account qualification file missing", code: "BF00110", wantCategory: BaofuErrorCategoryUserActionRequired, wantMessage: "资料信息不完整，请核对后重新提交", wantAction: "check_and_resubmit"},
		{name: "account identity verification", code: "BF00061", wantCategory: BaofuErrorCategoryUserActionRequired, wantMessage: "身份或银行卡信息核验未通过，请核对后重新提交", wantAction: "check_and_resubmit"},
		{name: "account system busy", code: "SYSTEM_INNER_ERROR", wantCategory: BaofuErrorCategoryRetryable, wantMessage: "支付通道处理中，请稍后重试", wantAction: "retry_later", wantRetryable: true},
		{name: "aggregate unopened product", code: "UNOPENED_PRODUCT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "支付通道配置待开通，请联系平台处理", wantAction: "contact_platform"},
		{name: "aggregate merchant report missing", code: "MERCHANT_NOT_REPORT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "商户微信支付通道待开通，请联系平台处理", wantAction: "contact_platform"},
		{name: "aggregate pay channel not enabled", code: "PAY_CHANNEL_NOT_SUPPORT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "商户微信支付通道待开通，请联系平台处理", wantAction: "contact_platform"},
		{name: "aggregate trade unknown", code: "TRADE_UNCONFIRMED", wantCategory: BaofuErrorCategoryRetryable, wantMessage: "交易结果处理中，请稍后查询", wantAction: "query_later", wantRetryable: true},
		{name: "aggregate duplicate order", code: "ORDER_EXIST", wantCategory: BaofuErrorCategoryRetryable, wantMessage: "支付订单已创建，请返回订单页查看支付状态", wantAction: "query_order", wantRetryable: true},
		{name: "aggregate risk refused", code: "RISK_REFUSED", wantCategory: BaofuErrorCategoryManualReview, wantMessage: "支付通道异常，请联系平台处理", wantAction: "contact_platform"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyBaofuError(tc.code, "上游原始错误描述")
			require.Equal(t, tc.wantCategory, got.Category)
			require.Equal(t, tc.wantMessage, got.PublicMessage)
			require.Equal(t, tc.wantAction, got.PublicAction)
			require.Equal(t, tc.wantRetryable, got.Retryable)
			require.NotContains(t, got.FrontendGuidance().Message, "上游原始")
		})
	}
}
