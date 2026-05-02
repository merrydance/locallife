package contracts

const (
	methodPost          = "POST"
	methodGet           = "GET"
	methodPut           = "PUT"
	methodDelete        = "DELETE"
	methodCallback      = "CALLBACK"
	methodLocalContract = "LOCAL"
)

// officialContractBinding is test-only metadata that binds official docs to local wire contracts.
// It must not be used by production code; production behavior lives in typed contracts and clients.
type officialContractBinding struct {
	Title             string
	URL               string
	Method            string
	Path              string
	RequestContracts  []string
	ResponseContracts []string
	StatusConstants   []string
	ErrorCodeSet      string
}

// officialContractCapabilityGroup groups contract bindings by business capability for tests.
type officialContractCapabilityGroup struct {
	Name     string
	Bindings []officialContractBinding
}

// officialContractCapabilityGroups groups test fixtures by ordinary-service-provider capability.
var officialContractCapabilityGroups = []officialContractCapabilityGroup{
	{Name: "特约商户进件与结算账户", Bindings: officialCapabilityApplyment},
	{Name: "商户开户意愿确认", Bindings: officialCapabilityAccountWillingness},
	{Name: "商户管控、商户平台处置通知与不活跃核实", Bindings: officialCapabilityMerchantManagement},
	{Name: "小程序支付", Bindings: officialCapabilityPayment},
	{Name: "小程序合单支付", Bindings: officialCapabilityCombinePayment},
	{Name: "订单退款", Bindings: officialCapabilityRefund},
	{Name: "分账", Bindings: officialCapabilityProfitSharing},
}

var officialCapabilityApplyment = []officialContractBinding{
	{Title: "特约商户进件-提交申请单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012719997.md", Method: methodPost, Path: "/v3/applyment4sub/applyment/", RequestContracts: []string{"ApplymentSubmitRequest"}, ResponseContracts: []string{"ApplymentSubmitResponse"}, ErrorCodeSet: "ApplymentSubmitDocumentedCodes"},
	{Title: "特约商户进件-申请单号查询申请状态", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012697052.md", Method: methodGet, Path: "/v3/applyment4sub/applyment/applyment_id/{applyment_id}", RequestContracts: []string{"ApplymentQueryByIDRequest"}, ResponseContracts: []string{"ApplymentQueryResponse"}, StatusConstants: []string{"ApplymentState"}, ErrorCodeSet: "ApplymentQueryDocumentedCodes"},
	{Title: "特约商户进件-业务申请编号查询申请状态", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012697168.md", Method: methodGet, Path: "/v3/applyment4sub/applyment/business_code/{business_code}", RequestContracts: []string{"ApplymentQueryByBusinessCodeRequest"}, ResponseContracts: []string{"ApplymentQueryResponse"}, StatusConstants: []string{"ApplymentState"}, ErrorCodeSet: "ApplymentQueryDocumentedCodes"},
	{Title: "特约商户进件-修改结算账户", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012761102.md", Method: methodPost, Path: "/v3/apply4sub/sub_merchants/{sub_mchid}/modify-settlement", RequestContracts: []string{"SettlementModifyRequest"}, ResponseContracts: []string{"SettlementModifyResponse"}, ErrorCodeSet: "SettlementModifyDocumentedCodes"},
	{Title: "特约商户进件-查询结算账户", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012761113.md", Method: methodGet, Path: "/v3/apply4sub/sub_merchants/{sub_mchid}/settlement", RequestContracts: []string{"SettlementQueryRequest"}, ResponseContracts: []string{"SettlementQueryResponse"}, StatusConstants: []string{"SettlementVerifyResult"}, ErrorCodeSet: "SettlementQueryDocumentedCodes"},
	{Title: "特约商户进件-查询结算账户修改申请状态", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012761120.md", Method: methodGet, Path: "/v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}", RequestContracts: []string{"SettlementModificationQueryRequest"}, ResponseContracts: []string{"SettlementModificationQueryResponse"}, StatusConstants: []string{"SettlementVerifyResult", "SettlementAuditResult"}, ErrorCodeSet: "SettlementModificationQueryDocumentedCodes"},
	{Title: "特约商户进件-图片上传", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012760490.md", Method: methodPost, Path: "/v3/merchant/media/upload", RequestContracts: []string{"MediaUploadRequestMultipart"}, ResponseContracts: []string{"MediaUploadResponse"}, ErrorCodeSet: "MerchantMediaUploadDocumentedCodes"},
}

var officialCapabilityAccountWillingness = []officialContractBinding{
	{Title: "商户开户意愿确认-提交申请单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012722388.md", Method: methodPost, Path: "/v3/apply4subject/applyment/", RequestContracts: []string{"AccountWillingnessSubmitRequest"}, ResponseContracts: []string{"AccountWillingnessSubmitResponse"}, ErrorCodeSet: "AccountWillingnessSubmitDocumentedCodes"},
	{Title: "商户开户意愿确认-撤销申请单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012697627.md", Method: methodPost, Path: "/v3/apply4subject/applyment/{business_code}/cancel", RequestContracts: []string{"AccountWillingnessCancelRequest"}, ResponseContracts: []string{"AccountWillingnessCancelResponse"}, ErrorCodeSet: "AccountWillingnessCancelDocumentedCodes"},
	{Title: "商户开户意愿确认-查询申请单审核结果", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012697715.md", Method: methodGet, Path: "/v3/apply4subject/applyment?business_code={business_code}", RequestContracts: []string{"AccountWillingnessQueryRequest"}, ResponseContracts: []string{"AccountWillingnessQueryResponse"}, StatusConstants: []string{"AccountWillingnessState"}, ErrorCodeSet: "AccountWillingnessQueryDocumentedCodes"},
	{Title: "商户开户意愿确认-获取商户开户意愿确认状态", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012467549.md", Method: methodGet, Path: "/v3/apply4subject/applyment/merchants/{sub_mchid}/state", RequestContracts: []string{"AccountAuthorizeStateRequest"}, ResponseContracts: []string{"AccountAuthorizeStateResponse"}, StatusConstants: []string{"AccountAuthorizeState"}, ErrorCodeSet: "AccountAuthorizeStateDocumentedCodes"},
	{Title: "商户开户意愿确认-图片上传", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012760509.md", Method: methodPost, Path: "/v3/merchant/media/upload", RequestContracts: []string{"MediaUploadRequestMultipart"}, ResponseContracts: []string{"MediaUploadResponse"}, ErrorCodeSet: "AccountWillingnessMediaUploadDocumentedCodes"},
}

var officialCapabilityMerchantManagement = []officialContractBinding{
	{Title: "商户平台处置通知-查询商户违规通知回调地址", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012471327.md", Method: methodGet, Path: "/v3/merchant-risk-manage/violation-notifications", ResponseContracts: []string{"ViolationNotificationConfigResponse"}, ErrorCodeSet: "ViolationNotificationConfigQueryDocumentedCodes"},
	{Title: "商户平台处置通知-修改商户违规通知回调地址", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012471330.md", Method: methodPut, Path: "/v3/merchant-risk-manage/violation-notifications", RequestContracts: []string{"ViolationNotificationConfigRequest"}, ResponseContracts: []string{"ViolationNotificationConfigResponse"}, ErrorCodeSet: "ViolationNotificationConfigUpdateDocumentedCodes"},
	{Title: "商户平台处置通知-创建商户违规通知回调地址", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012471333.md", Method: methodPost, Path: "/v3/merchant-risk-manage/violation-notifications", RequestContracts: []string{"ViolationNotificationConfigRequest"}, ResponseContracts: []string{"ViolationNotificationConfigResponse"}, ErrorCodeSet: "ViolationNotificationConfigCreateDocumentedCodes"},
	{Title: "商户平台处置通知-删除商户违规通知回调地址", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012471334.md", Method: methodDelete, Path: "/v3/merchant-risk-manage/violation-notifications", RequestContracts: []string{"NoRequestBody"}, ResponseContracts: []string{"NoResponseBody"}, ErrorCodeSet: "ViolationNotificationConfigDeleteDocumentedCodes"},
	{Title: "商户平台处置通知-商户平台处置记录回调通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012079216.md", Method: methodCallback, Path: "configured merchant violation notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"MerchantViolationNotificationPayload", "WechatErrorResponse"}},
	{Title: "商户被管控能力及原因查询-查询子商户管控情况", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012803072.md", Method: methodGet, Path: "/v3/mch-operation-manage/merchant-limitations/sub-mchid/{sub_mchid}", RequestContracts: []string{"MerchantLimitationQueryRequest"}, ResponseContracts: []string{"MerchantLimitationQueryResponse"}, StatusConstants: []string{"MerchantLimitedFunction"}, ErrorCodeSet: "MerchantLimitationQueryDocumentedCodes"},
	{Title: "不活跃商户身份核实-发起不活跃商户身份核实", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012471357.md", Method: methodPost, Path: "/v3/compliance/inactive-merchant-identity-verification/merchants", RequestContracts: []string{"InactiveMerchantIdentityVerificationCreateRequest"}, ResponseContracts: []string{"InactiveMerchantIdentityVerificationCreateResponse"}, ErrorCodeSet: "InactiveMerchantIdentityVerificationCreateDocumentedCodes"},
	{Title: "不活跃商户身份核实-查询不活跃商户身份核实结果", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012471359.md", Method: methodGet, Path: "/v3/compliance/inactive-merchant-identity-verification/merchants/{sub_mchid}/verifications/{verification_id}", RequestContracts: []string{"InactiveMerchantIdentityVerificationQueryRequest"}, ResponseContracts: []string{"InactiveMerchantIdentityVerificationQueryResponse"}, StatusConstants: []string{"InactiveMerchantIdentityVerificationState"}, ErrorCodeSet: "InactiveMerchantIdentityVerificationQueryDocumentedCodes"},
}

var officialCapabilityPayment = []officialContractBinding{
	{Title: "小程序支付-JSAPI/小程序下单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012759974.md", Method: methodPost, Path: "/v3/pay/partner/transactions/jsapi", RequestContracts: []string{"PaymentPrepayRequest"}, ResponseContracts: []string{"PaymentPrepayResponse"}, ErrorCodeSet: "PaymentPrepayDocumentedCodes"},
	{Title: "小程序支付-支付成功回调通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012085801.md", Method: methodCallback, Path: "configured payment notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"PaymentNotificationPayload", "WechatErrorResponse"}, StatusConstants: []string{"PaymentTradeState"}},
	{Title: "小程序支付-微信支付订单号查询订单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012738973.md", Method: methodGet, Path: "/v3/pay/partner/transactions/id/{transaction_id}", RequestContracts: []string{"PaymentQueryRequest"}, ResponseContracts: []string{"PaymentQueryResponse"}, StatusConstants: []string{"PaymentTradeState"}, ErrorCodeSet: "PaymentQueryDocumentedCodes"},
	{Title: "小程序支付-商户订单号查询订单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012760115.md", Method: methodGet, Path: "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}", RequestContracts: []string{"PaymentQueryRequest"}, ResponseContracts: []string{"PaymentQueryResponse"}, StatusConstants: []string{"PaymentTradeState"}, ErrorCodeSet: "PaymentQueryDocumentedCodes"},
	{Title: "小程序支付-关闭订单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012760108.md", Method: methodPost, Path: "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close", RequestContracts: []string{"PaymentCloseRequest"}, ErrorCodeSet: "PaymentCloseDocumentedCodes"},
	{Title: "小程序支付-申请退款", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012760121.md", Method: methodPost, Path: "/v3/refund/domestic/refunds", RequestContracts: []string{"RefundCreateRequest"}, ResponseContracts: []string{"RefundResponse"}, StatusConstants: []string{"RefundStatus"}, ErrorCodeSet: "RefundCreateDocumentedCodes"},
	{Title: "小程序支付-查询单笔退款（通过商户退款单号）", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012760128.md", Method: methodGet, Path: "/v3/refund/domestic/refunds/{out_refund_no}", RequestContracts: []string{"RefundQueryRequest"}, ResponseContracts: []string{"RefundResponse"}, StatusConstants: []string{"RefundStatus"}, ErrorCodeSet: "RefundQueryDocumentedCodes"},
	{Title: "小程序支付-退款结果回调通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012085802.md", Method: methodCallback, Path: "configured refund notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"RefundNotificationPayload", "WechatErrorResponse"}, StatusConstants: []string{"RefundStatus"}},
}

var officialCapabilityCombinePayment = []officialContractBinding{
	{Title: "小程序合单支付-小程序合单下单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012758246.md", Method: methodPost, Path: "/v3/combine-transactions/jsapi", RequestContracts: []string{"CombinePrepayRequest"}, ResponseContracts: []string{"CombinePrepayResponse"}, ErrorCodeSet: "CombinePrepayDocumentedCodes"},
	{Title: "小程序合单支付-小程序调起支付", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012166847.md", Method: methodLocalContract, Path: "wx.requestPayment parameters", ResponseContracts: []string{"JSAPIPayParams"}},
	{Title: "小程序合单支付-查询合单订单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013462520.md", Method: methodGet, Path: "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}", RequestContracts: []string{"CombineQueryRequest"}, ResponseContracts: []string{"CombineQueryResponse"}, StatusConstants: []string{"PaymentTradeState"}, ErrorCodeSet: "CombineQueryDocumentedCodes"},
	{Title: "小程序合单支付-关闭合单订单", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013462566.md", Method: methodPost, Path: "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close", RequestContracts: []string{"CombineCloseRequest"}, ErrorCodeSet: "CombineCloseDocumentedCodes"},
	{Title: "小程序合单支付-合单订单支付成功回调通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013462574.md", Method: methodCallback, Path: "configured combine payment notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"CombinePaymentNotificationPayload", "WechatErrorResponse"}, StatusConstants: []string{"PaymentTradeState"}},
	{Title: "小程序合单支付-申请退款", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013462579.md", Method: methodPost, Path: "/v3/refund/domestic/refunds", RequestContracts: []string{"RefundCreateRequest"}, ResponseContracts: []string{"RefundResponse"}, StatusConstants: []string{"RefundStatus"}, ErrorCodeSet: "RefundCreateDocumentedCodes"},
	{Title: "小程序合单支付-查询单笔退款（按商户退款单号）", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013462581.md", Method: methodGet, Path: "/v3/refund/domestic/refunds/{out_refund_no}", RequestContracts: []string{"RefundQueryRequest"}, ResponseContracts: []string{"RefundResponse"}, StatusConstants: []string{"RefundStatus"}, ErrorCodeSet: "RefundQueryDocumentedCodes"},
	{Title: "小程序合单支付-退款结果回调通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013462586.md", Method: methodCallback, Path: "configured refund notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"RefundNotificationPayload", "WechatErrorResponse"}, StatusConstants: []string{"RefundStatus"}},
}

var officialCapabilityRefund = []officialContractBinding{
	{Title: "订单退款-申请退款", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013080625.md", Method: methodPost, Path: "/v3/refund/domestic/refunds", RequestContracts: []string{"RefundCreateRequest"}, ResponseContracts: []string{"RefundResponse"}, StatusConstants: []string{"RefundStatus"}, ErrorCodeSet: "RefundCreateDocumentedCodes"},
	{Title: "订单退款-查询单笔退款（通过商户退款单号）", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013080626.md", Method: methodGet, Path: "/v3/refund/domestic/refunds/{out_refund_no}", RequestContracts: []string{"RefundQueryRequest"}, ResponseContracts: []string{"RefundResponse"}, StatusConstants: []string{"RefundStatus"}, ErrorCodeSet: "RefundQueryDocumentedCodes"},
	{Title: "订单退款-退款结果通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4013080628.md", Method: methodCallback, Path: "configured refund notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"RefundNotificationPayload", "WechatErrorResponse"}, StatusConstants: []string{"RefundStatus"}},
}

var officialCapabilityProfitSharing = []officialContractBinding{
	{Title: "分账-请求分账", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012690683.md", Method: methodPost, Path: "/v3/profitsharing/orders", RequestContracts: []string{"ProfitSharingOrderRequest"}, ResponseContracts: []string{"ProfitSharingOrderResponse"}, StatusConstants: []string{"ProfitSharingOrderState", "ProfitSharingReceiverResult", "ProfitSharingFailReason"}, ErrorCodeSet: "ProfitSharingCreateDocumentedCodes"},
	{Title: "分账-查询分账结果", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012466850.md", Method: methodGet, Path: "/v3/profitsharing/orders/{out_order_no}", RequestContracts: []string{"ProfitSharingQueryRequest"}, ResponseContracts: []string{"ProfitSharingOrderResponse"}, StatusConstants: []string{"ProfitSharingOrderState", "ProfitSharingReceiverResult", "ProfitSharingFailReason"}, ErrorCodeSet: "ProfitSharingQueryDocumentedCodes"},
	{Title: "分账-请求分账回退", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012466854.md", Method: methodPost, Path: "/v3/profitsharing/return-orders", RequestContracts: []string{"ProfitSharingReturnRequest"}, ResponseContracts: []string{"ProfitSharingReturnResponse"}, StatusConstants: []string{"ProfitSharingReturnState"}, ErrorCodeSet: "ProfitSharingReturnCreateDocumentedCodes"},
	{Title: "分账-查询分账回退结果", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012466858.md", Method: methodGet, Path: "/v3/profitsharing/return-orders/{out_return_no}", RequestContracts: []string{"ProfitSharingReturnQueryRequest"}, ResponseContracts: []string{"ProfitSharingReturnResponse"}, StatusConstants: []string{"ProfitSharingReturnState"}, ErrorCodeSet: "ProfitSharingReturnQueryDocumentedCodes"},
	{Title: "分账-解冻剩余资金", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012466860.md", Method: methodPost, Path: "/v3/profitsharing/orders/unfreeze", RequestContracts: []string{"ProfitSharingUnfreezeRequest"}, ResponseContracts: []string{"ProfitSharingUnfreezeResponse"}, StatusConstants: []string{"ProfitSharingOrderState"}, ErrorCodeSet: "ProfitSharingUnfreezeDocumentedCodes"},
	{Title: "分账-查询剩余待分金额", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012457927.md", Method: methodGet, Path: "/v3/profitsharing/transactions/{transaction_id}/amounts", RequestContracts: []string{"ProfitSharingRemainingAmountRequest"}, ResponseContracts: []string{"ProfitSharingRemainingAmountResponse"}, ErrorCodeSet: "ProfitSharingRemainingAmountDocumentedCodes"},
	{Title: "分账-添加分账接收方", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012690944.md", Method: methodPost, Path: "/v3/profitsharing/receivers/add", RequestContracts: []string{"ProfitSharingReceiverAddRequest"}, ResponseContracts: []string{"ProfitSharingReceiverResponse"}, StatusConstants: []string{"ReceiverType", "ProfitSharingReceiverRelationType"}, ErrorCodeSet: "ProfitSharingReceiverAddDocumentedCodes"},
	{Title: "分账-删除分账接收方", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012466868.md", Method: methodPost, Path: "/v3/profitsharing/receivers/delete", RequestContracts: []string{"ProfitSharingReceiverDeleteRequest"}, ResponseContracts: []string{"ProfitSharingReceiverResponse"}, StatusConstants: []string{"ReceiverType"}, ErrorCodeSet: "ProfitSharingReceiverDeleteDocumentedCodes"},
	{Title: "分账-分账动账通知", URL: "https://pay.weixin.qq.com/doc/v3/partner/4012075216.md", Method: methodCallback, Path: "configured profit sharing notify_url", RequestContracts: []string{"NotificationRequest", "NotificationResource"}, ResponseContracts: []string{"ProfitSharingNotificationPayload", "WechatErrorResponse"}, StatusConstants: []string{"ProfitSharingReceiverResult"}},
}

// officialContractBindings flattens test-only bindings for contract anti-drift checks.
var officialContractBindings = flattenOfficialContractBindings(officialContractCapabilityGroups)

func flattenOfficialContractBindings(groups []officialContractCapabilityGroup) []officialContractBinding {
	var bindings []officialContractBinding
	for _, group := range groups {
		bindings = append(bindings, group.Bindings...)
	}
	return bindings
}

func officialContractBindingByTitle(title string) (officialContractBinding, bool) {
	for _, binding := range officialContractBindings {
		if binding.Title == title {
			return binding, true
		}
	}
	return officialContractBinding{}, false
}
