package errorcodes

import (
	"reflect"
	"testing"
)

var officialEndpointCodeSets = map[string]DocumentedCodeSet{
	"特约商户进件-提交申请单":            ApplymentSubmitDocumentedCodes,
	"特约商户进件-申请单号查询申请状态":       ApplymentQueryDocumentedCodes,
	"特约商户进件-业务申请编号查询申请状态":     ApplymentQueryDocumentedCodes,
	"特约商户进件-修改结算账户":           SettlementModifyDocumentedCodes,
	"特约商户进件-查询结算账户":           SettlementQueryDocumentedCodes,
	"特约商户进件-查询结算账户修改申请状态":     SettlementModificationQueryDocumentedCodes,
	"特约商户进件-图片上传":             MerchantMediaUploadDocumentedCodes,
	"商户开户意愿确认-提交申请单":          AccountWillingnessSubmitDocumentedCodes,
	"商户开户意愿确认-撤销申请单":          AccountWillingnessCancelDocumentedCodes,
	"商户开户意愿确认-查询申请单审核结果":      AccountWillingnessQueryDocumentedCodes,
	"商户开户意愿确认-获取商户开户意愿确认状态":   AccountAuthorizeStateDocumentedCodes,
	"商户开户意愿确认-图片上传":           AccountWillingnessMediaUploadDocumentedCodes,
	"商户平台处置通知-查询商户违规通知回调地址":   ViolationNotificationConfigQueryDocumentedCodes,
	"商户平台处置通知-修改商户违规通知回调地址":   ViolationNotificationConfigUpdateDocumentedCodes,
	"商户平台处置通知-创建商户违规通知回调地址":   ViolationNotificationConfigCreateDocumentedCodes,
	"商户平台处置通知-删除商户违规通知回调地址":   ViolationNotificationConfigDeleteDocumentedCodes,
	"商户被管控能力及原因查询-查询子商户管控情况":  MerchantLimitationQueryDocumentedCodes,
	"不活跃商户身份核实-发起不活跃商户身份核实":   InactiveMerchantIdentityVerificationCreateDocumentedCodes,
	"不活跃商户身份核实-查询不活跃商户身份核实结果": InactiveMerchantIdentityVerificationQueryDocumentedCodes,
	"小程序支付-JSAPI/小程序下单":       PaymentPrepayDocumentedCodes,
	"小程序支付-微信支付订单号查询订单":       PaymentQueryDocumentedCodes,
	"小程序支付-商户订单号查询订单":         PaymentQueryDocumentedCodes,
	"小程序支付-关闭订单":              PaymentCloseDocumentedCodes,
	"小程序支付-申请退款":              RefundCreateDocumentedCodes,
	"小程序支付-查询单笔退款（通过商户退款单号）":  RefundQueryDocumentedCodes,
	"小程序合单支付-小程序合单下单":         CombinePrepayDocumentedCodes,
	"小程序合单支付-查询合单订单":          CombineQueryDocumentedCodes,
	"小程序合单支付-关闭合单订单":          CombineCloseDocumentedCodes,
	"小程序合单支付-申请退款":            RefundCreateDocumentedCodes,
	"小程序合单支付-查询单笔退款（按商户退款单号）": RefundQueryDocumentedCodes,
	"订单退款-申请退款":               RefundCreateDocumentedCodes,
	"订单退款-查询单笔退款（通过商户退款单号）":   RefundQueryDocumentedCodes,
	"分账-请求分账":                 ProfitSharingCreateDocumentedCodes,
	"分账-查询分账结果":               ProfitSharingQueryDocumentedCodes,
	"分账-请求分账回退":               ProfitSharingReturnCreateDocumentedCodes,
	"分账-查询分账回退结果":             ProfitSharingReturnQueryDocumentedCodes,
	"分账-解冻剩余资金":               ProfitSharingUnfreezeDocumentedCodes,
	"分账-查询剩余待分金额":             ProfitSharingRemainingAmountDocumentedCodes,
	"分账-添加分账接收方":              ProfitSharingReceiverAddDocumentedCodes,
	"分账-删除分账接收方":              ProfitSharingReceiverDeleteDocumentedCodes,
}

func TestOfficialEndpointCodeSetsMatchWechatOrdinaryDocs(t *testing.T) {
	tests := map[string][]string{
		"特约商户进件-提交申请单":            {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPLYMENT_NOTEXIST", "PROCESSING", "NO_AUTH", "REQUEST_BLOCKED", "RATE_LIMITED"},
		"特约商户进件-申请单号查询申请状态":       {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPLYMENT_NOT_EXIST", "NO_AUTH", "PROCESSING", "RATE_LIMITED"},
		"特约商户进件-业务申请编号查询申请状态":     {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPLYMENT_NOT_EXIST", "NO_AUTH", "PROCESSING", "RATE_LIMITED"},
		"特约商户进件-修改结算账户":           {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "FREQENCY_LIMIT"},
		"特约商户进件-查询结算账户":           {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH"},
		"特约商户进件-查询结算账户修改申请状态":     {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "ORDER_NOT_EXIST"},
		"特约商户进件-图片上传":             {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "FREQUENCY_LIMIT_EXCEED", "NO_AUTH"},
		"商户开户意愿确认-提交申请单":          {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH"},
		"商户开户意愿确认-撤销申请单":          {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH"},
		"商户开户意愿确认-查询申请单审核结果":      {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH"},
		"商户开户意愿确认-获取商户开户意愿确认状态":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH"},
		"商户开户意愿确认-图片上传":           {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "FREQUENCY_LIMIT_EXCEED", "NO_AUTH"},
		"商户平台处置通知-查询商户违规通知回调地址":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "NOT_FOUND", "FREQUENCY_LIMITED"},
		"商户平台处置通知-修改商户违规通知回调地址":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "NOT_FOUND", "FREQUENCY_LIMITED"},
		"商户平台处置通知-创建商户违规通知回调地址":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "ALREADY_EXISTS", "FREQUENCY_LIMITED"},
		"商户平台处置通知-删除商户违规通知回调地址":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "NOT_FOUND", "FREQUENCY_LIMITED"},
		"商户被管控能力及原因查询-查询子商户管控情况":  {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "RATELIMIT_EXCEEDED"},
		"不活跃商户身份核实-发起不活跃商户身份核实":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "FREQUENCY_LIMIT_EXCEED"},
		"不活跃商户身份核实-查询不活跃商户身份核实结果": {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "NOT_FOUND", "FREQUENCY_LIMIT_EXCEED"},
		"小程序支付-JSAPI/小程序下单":       {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPID_MCHID_NOT_MATCH", "MCH_NOT_EXISTS", "NO_AUTH", "OUT_TRADE_NO_USED", "FREQUENCY_LIMITED"},
		"小程序支付-微信支付订单号查询订单":       {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "MCH_NOT_EXISTS", "RULE_LIMIT", "TRADE_ERROR", "ORDER_NOT_EXIST", "FREQUENCY_LIMITED"},
		"小程序支付-商户订单号查询订单":         {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "MCH_NOT_EXISTS", "RULE_LIMIT", "TRADE_ERROR", "ORDER_NOT_EXIST", "FREQUENCY_LIMITED"},
		"小程序支付-关闭订单":              {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "MCH_NOT_EXISTS", "RULE_LIMIT", "TRADE_ERROR", "FREQUENCY_LIMITED"},
		"小程序支付-申请退款":              {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NOT_ENOUGH", "USER_ACCOUNT_ABNORMAL", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS", "FREQUENCY_LIMITED"},
		"小程序支付-查询单笔退款（通过商户退款单号）":  {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS"},
		"小程序合单支付-小程序合单下单":         {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPID_MCHID_NOT_MATCH", "MCH_NOT_EXISTS", "ORDER_CLOSED", "NOAUTH", "OUT_TRADE_NO_USED", "RULELIMIT", "FREQUENCY_LIMITED", "OPENID_MISMATCH", "SYSTEMERROR"},
		"小程序合单支付-查询合单订单":          {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPID_MCHID_NOT_MATCH", "MCH_NOT_EXISTS", "ORDER_CLOSED", "NOAUTH", "OUT_TRADE_NO_USED", "RULELIMIT", "FREQUENCY_LIMITED", "SYSTEMERROR"},
		"小程序合单支付-关闭合单订单":          {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "APPID_MCHID_NOT_MATCH", "MCH_NOT_EXISTS", "ORDER_CLOSED", "NOAUTH", "OUT_TRADE_NO_USED", "RULELIMIT", "FREQUENCY_LIMITED", "SYSTEMERROR"},
		"小程序合单支付-申请退款":            {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NOT_ENOUGH", "USER_ACCOUNT_ABNORMAL", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS", "FREQUENCY_LIMITED"},
		"小程序合单支付-查询单笔退款（按商户退款单号）": {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS"},
		"订单退款-申请退款":               {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NOT_ENOUGH", "USER_ACCOUNT_ABNORMAL", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS", "FREQUENCY_LIMITED"},
		"订单退款-查询单笔退款（通过商户退款单号）":   {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "MCH_NOT_EXISTS", "RESOURCE_NOT_EXISTS"},
		"分账-请求分账":                 {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "RULE_LIMIT", "NOT_ENOUGH", "FREQUENCY_LIMITED"},
		"分账-查询分账结果":               {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "RESOURCE_NOT_EXISTS", "FREQUENCY_LIMITED"},
		"分账-请求分账回退":               {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "FREQUENCY_LIMITED"},
		"分账-查询分账回退结果":             {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "RESOURCE_NOT_EXISTS", "FREQUENCY_LIMITED"},
		"分账-解冻剩余资金":               {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "NOT_ENOUGH", "FREQUENCY_LIMITED"},
		"分账-查询剩余待分金额":             {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "RESOURCE_NOT_EXISTS", "FREQUENCY_LIMITED"},
		"分账-添加分账接收方":              {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "FREQUENCY_LIMITED"},
		"分账-删除分账接收方":              {"PARAM_ERROR", "INVALID_REQUEST", "SIGN_ERROR", "SYSTEM_ERROR", "NO_AUTH", "FREQUENCY_LIMITED"},
	}

	if len(officialEndpointCodeSets) != len(tests) {
		t.Fatalf("official endpoint code set count = %d, want %d", len(officialEndpointCodeSets), len(tests))
	}
	for title, want := range tests {
		got, ok := officialEndpointCodeSets[title]
		if !ok {
			t.Fatalf("missing official endpoint code set for %s", title)
		}
		if !reflect.DeepEqual(got.OfficialCodes, want) {
			t.Fatalf("%s codes = %#v, want %#v", title, got.OfficialCodes, want)
		}
		for _, code := range want {
			if !got.Has(code) {
				t.Fatalf("%s set should match official code %s", title, code)
			}
		}
	}
}

func TestOfficialCodeSetAliasesMatchWechatLegacySpellings(t *testing.T) {
	if !CombinePrepayDocumentedCodes.Has("NO_AUTH") || !CombinePrepayDocumentedCodes.Has("NOAUTH") {
		t.Fatal("combine create set must match both official NOAUTH and canonical NO_AUTH")
	}
	if !CombinePrepayDocumentedCodes.Has("RULE_LIMIT") || !CombinePrepayDocumentedCodes.Has("RULELIMIT") {
		t.Fatal("combine create set must match both official RULELIMIT and canonical RULE_LIMIT")
	}
	if !SettlementModifyDocumentedCodes.Has("FREQENCY_LIMIT") || !SettlementModifyDocumentedCodes.Has("FREQUENCY_LIMIT") {
		t.Fatal("settlement modify set must match official FREQENCY_LIMIT and compatibility FREQUENCY_LIMIT")
	}
}
