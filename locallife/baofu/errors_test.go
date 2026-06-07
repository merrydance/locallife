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
		{"PARAM_ERROR", BaofuErrorCategoryUserActionRequired, "资料信息不完整，请核对后重新提交：开户资料缺失"},
		{"BF0020", BaofuErrorCategoryUserActionRequired, "资料信息不完整，请核对后重新提交：统一社会信用代码格式不正确"},
		{"MERCHANT_NOT_REPORTED", BaofuErrorCategoryPlatformConfiguration, "商户微信支付通道待开通，请联系平台处理"},
		{"MERCHANT_REPORT_LIMIT", BaofuErrorCategoryPlatformConfiguration, "该主体已有微信渠道报备记录，请联系平台核对开通状态"},
		{"PAY_CHANNEL_NOT_SUPPORT", BaofuErrorCategoryPlatformConfiguration, "商户微信支付通道待开通，请联系平台处理"},
		{"SYSTEM_BUSY", BaofuErrorCategoryRetryable, "支付通道处理中，请稍后重试"},
		{"UNKNOWN", BaofuErrorCategoryManualReview, "支付通道异常，请联系平台处理"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			upstreamMessage := "开户资料缺失"
			if tc.code == "BF0020" {
				upstreamMessage = "统一社会信用代码格式不正确"
			}
			got := ClassifyBaofuError(tc.code, upstreamMessage)
			require.Equal(t, tc.wantCategory, got.Category)
			require.Equal(t, tc.wantPublic, got.PublicMessage)
		})
	}
}

func TestClassifyBaofuOfficialErrorTables(t *testing.T) {
	cases := []struct {
		name            string
		code            string
		upstreamMessage string
		wantCategory    BaofuErrorCategory
		wantMessage     string
		wantAction      string
		wantRetryable   bool
	}{
		{name: "account parameter", code: "BF0001", upstreamMessage: "开户资料缺失", wantCategory: BaofuErrorCategoryUserActionRequired, wantMessage: "资料信息不完整，请核对后重新提交：开户资料缺失", wantAction: "check_and_resubmit"},
		{name: "account qualification file missing", code: "BF00110", upstreamMessage: "资质文件缺失", wantCategory: BaofuErrorCategoryUserActionRequired, wantMessage: "资料信息不完整，请核对后重新提交：资质文件缺失", wantAction: "check_and_resubmit"},
		{name: "account identity verification", code: "BF00061", upstreamMessage: "身份证号码不合法", wantCategory: BaofuErrorCategoryUserActionRequired, wantMessage: "身份或银行卡信息核验未通过，请核对后重新提交：身份证号码不合法", wantAction: "check_and_resubmit"},
		{name: "account system busy", code: "SYSTEM_INNER_ERROR", wantCategory: BaofuErrorCategoryRetryable, wantMessage: "支付通道处理中，请稍后重试", wantAction: "retry_later", wantRetryable: true},
		{name: "aggregate unopened product", code: "UNOPENED_PRODUCT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "支付通道配置待开通，请联系平台处理", wantAction: "contact_platform"},
		{name: "aggregate merchant report missing", code: "MERCHANT_NOT_REPORT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "商户微信支付通道待开通，请联系平台处理", wantAction: "contact_platform"},
		{name: "aggregate merchant report limit", code: "MERCHANT_REPORT_LIMIT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "该主体已有微信渠道报备记录，请联系平台核对开通状态", wantAction: "contact_platform"},
		{name: "aggregate pay channel not enabled", code: "PAY_CHANNEL_NOT_SUPPORT", wantCategory: BaofuErrorCategoryPlatformConfiguration, wantMessage: "商户微信支付通道待开通，请联系平台处理", wantAction: "contact_platform"},
		{name: "aggregate trade unknown", code: "TRADE_UNCONFIRMED", wantCategory: BaofuErrorCategoryRetryable, wantMessage: "交易结果处理中，请稍后查询", wantAction: "query_later", wantRetryable: true},
		{name: "aggregate duplicate order", code: "ORDER_EXIST", wantCategory: BaofuErrorCategoryRetryable, wantMessage: "支付订单已创建，请返回订单页查看支付状态", wantAction: "query_order", wantRetryable: true},
		{name: "aggregate risk refused", code: "RISK_REFUSED", wantCategory: BaofuErrorCategoryManualReview, wantMessage: "支付通道异常，请联系平台处理", wantAction: "contact_platform"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyBaofuError(tc.code, tc.upstreamMessage)
			require.Equal(t, tc.wantCategory, got.Category)
			require.Equal(t, tc.wantMessage, got.PublicMessage)
			require.Equal(t, tc.wantAction, got.PublicAction)
			require.Equal(t, tc.wantRetryable, got.Retryable)
		})
	}
}

func TestBaofuUpstreamMessageSanitizationForRecord(t *testing.T) {
	message := "通道返回身份证 110101199001010011 银行卡 6222020202020202 手机 13800138000 login_no=LLBFOO0000000999 contractNo=CP_SECRET_123 merId=102004465 terId=200005200 memberId=100030218 sub_mch_id=1900000118"

	got := SanitizeUpstreamMessageForRecord(message)

	require.NotEmpty(t, got)
	require.NotContains(t, got, "110101199001010011")
	require.NotContains(t, got, "6222020202020202")
	require.NotContains(t, got, "13800138000")
	require.NotContains(t, got, "LLBFOO0000000999")
	require.NotContains(t, got, "CP_SECRET_123")
	require.NotContains(t, got, "102004465")
	require.NotContains(t, got, "200005200")
	require.NotContains(t, got, "100030218")
	require.NotContains(t, got, "1900000118")
	require.Contains(t, got, "110101********0011")
	require.Contains(t, got, "************0202")
	require.Contains(t, got, "138****8000")
	require.Contains(t, got, "login_no=<redacted>")
	require.Contains(t, got, "contractNo=<redacted>")
	require.Contains(t, got, "merId=<redacted>")
	require.Contains(t, got, "terId=<redacted>")
	require.Contains(t, got, "memberId=<redacted>")
	require.Contains(t, got, "sub_mch_id=<redacted>")
}

func TestBaofuUserVisibleMessageIncludesActionableProviderReason(t *testing.T) {
	got := ClassifyBaofuError("BF0020", "统一社会信用代码格式不正确")

	require.Equal(t, BaofuErrorCategoryUserActionRequired, got.Category)
	require.Equal(t, "资料信息不完整，请核对后重新提交：统一社会信用代码格式不正确", got.PublicMessage)
	require.Equal(t, "check_and_resubmit", got.PublicAction)
	require.Equal(t, got.PublicMessage, got.FrontendGuidance().Message)
	require.Equal(t, "资料信息不完整，请核对后重新提交：统一社会信用代码格式不正确，check_and_resubmit", BaofuCommandMessage("BF0020", "统一社会信用代码格式不正确"))
}

func TestBaofuUserVisibleMessageRedactsSensitiveProviderReason(t *testing.T) {
	got := ClassifyBaofuError("ID_CARD_CHECK_FAILED", "身份证号码 110101199001010011 不合法")

	require.Equal(t, "身份或银行卡信息核验未通过，请核对后重新提交：身份证号码 110101********0011 不合法", got.PublicMessage)
	require.NotContains(t, got.PublicMessage, "110101199001010011")
}

func TestBaofuUserVisibleMessageKeepsDuplicateAccountReasonSafe(t *testing.T) {
	got := ClassifyBaofuError("BF00060", "login_no=LLBFOO0000000999 已存在")

	require.Equal(t, BaofuErrorCategoryUserActionRequired, got.Category)
	require.Equal(t, "该主体已存在宝付开户记录，请联系平台核对账户状态", got.PublicMessage)
	require.NotContains(t, got.PublicMessage, "LLBFOO0000000999")
	require.NotContains(t, BaofuCommandMessage("BF00060", "login_no=LLBFOO0000000999 已存在"), "LLBFOO0000000999")
}

func TestBaofuUserVisibleUpstreamReasonRejectsInternalIdentifiers(t *testing.T) {
	require.Empty(t, UserVisibleUpstreamReason("PARAM_ERROR", "appid wx123456 授权目录错误"))
	require.Empty(t, UserVisibleUpstreamReason("PARAM_ERROR", "merId=102004465 terId=200005200 参数错误"))
	require.Empty(t, UserVisibleUpstreamReason("PARAM_ERROR", "login_no=LLBFOO0000000999 已存在"))
}

func TestBaofuCommandMessageRecordsCleanUserActionableReason(t *testing.T) {
	require.Equal(t,
		"资料信息不完整，请核对后重新提交：退款金额超过可退金额，check_and_resubmit",
		BaofuCommandMessage("REFUND_AMT_EXCEEDS", "退款金额超过可退金额"),
	)
}
