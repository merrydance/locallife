package errorcodes

type CapabilityID string

type EndpointID string

const (
	CapabilityApplyment          CapabilityID = "applyment"
	CapabilityAccountWillingness CapabilityID = "account_willingness"
	CapabilityMerchantManagement CapabilityID = "merchant_management"
	CapabilityPayment            CapabilityID = "payment"
	CapabilityCombinePayment     CapabilityID = "combine_payment"
	CapabilityRefund             CapabilityID = "refund"
	CapabilityProfitSharing      CapabilityID = "profit_sharing"
)

const (
	EndpointApplymentSubmit                    EndpointID = "applyment.submit"
	EndpointApplymentQueryByID                 EndpointID = "applyment.query_by_id"
	EndpointApplymentQueryByBusinessCode       EndpointID = "applyment.query_by_business_code"
	EndpointSettlementModify                   EndpointID = "applyment.settlement_modify"
	EndpointSettlementQuery                    EndpointID = "applyment.settlement_query"
	EndpointSettlementModificationQuery        EndpointID = "applyment.settlement_modification_query"
	EndpointMerchantMediaUpload                EndpointID = "applyment.media_upload"
	EndpointAccountWillingnessSubmit           EndpointID = "account_willingness.submit"
	EndpointAccountWillingnessCancel           EndpointID = "account_willingness.cancel"
	EndpointAccountWillingnessQuery            EndpointID = "account_willingness.query"
	EndpointAccountAuthorizeState              EndpointID = "account_willingness.authorize_state"
	EndpointAccountWillingnessMediaUpload      EndpointID = "account_willingness.media_upload"
	EndpointViolationNotificationConfigQuery   EndpointID = "merchant_management.violation_notification_config_query"
	EndpointViolationNotificationConfigUpdate  EndpointID = "merchant_management.violation_notification_config_update"
	EndpointViolationNotificationConfigCreate  EndpointID = "merchant_management.violation_notification_config_create"
	EndpointViolationNotificationConfigDelete  EndpointID = "merchant_management.violation_notification_config_delete"
	EndpointMerchantLimitationQuery            EndpointID = "merchant_management.limitation_query"
	EndpointInactiveMerchantVerificationCreate EndpointID = "merchant_management.inactive_verification_create"
	EndpointInactiveMerchantVerificationQuery  EndpointID = "merchant_management.inactive_verification_query"
	EndpointPaymentPrepay                      EndpointID = "payment.prepay"
	EndpointPaymentQueryByTransactionID        EndpointID = "payment.query_by_transaction_id"
	EndpointPaymentQueryByOutTradeNo           EndpointID = "payment.query_by_out_trade_no"
	EndpointPaymentClose                       EndpointID = "payment.close"
	EndpointPaymentRefundCreate                EndpointID = "payment.refund_create"
	EndpointPaymentRefundQuery                 EndpointID = "payment.refund_query"
	EndpointCombinePrepay                      EndpointID = "combine_payment.prepay"
	EndpointCombineQuery                       EndpointID = "combine_payment.query"
	EndpointCombineClose                       EndpointID = "combine_payment.close"
	EndpointCombineRefundCreate                EndpointID = "combine_payment.refund_create"
	EndpointCombineRefundQuery                 EndpointID = "combine_payment.refund_query"
	EndpointRefundCreate                       EndpointID = "refund.create"
	EndpointRefundQuery                        EndpointID = "refund.query"
	EndpointProfitSharingCreate                EndpointID = "profit_sharing.create"
	EndpointProfitSharingQuery                 EndpointID = "profit_sharing.query"
	EndpointProfitSharingReturnCreate          EndpointID = "profit_sharing.return_create"
	EndpointProfitSharingReturnQuery           EndpointID = "profit_sharing.return_query"
	EndpointProfitSharingUnfreeze              EndpointID = "profit_sharing.unfreeze"
	EndpointProfitSharingRemainingAmount       EndpointID = "profit_sharing.remaining_amount"
	EndpointProfitSharingReceiverAdd           EndpointID = "profit_sharing.receiver_add"
	EndpointProfitSharingReceiverDelete        EndpointID = "profit_sharing.receiver_delete"
)

type CapabilityCodeSetGroup struct {
	ID        CapabilityID
	Name      string
	Endpoints []EndpointID
}

var capabilityCodeSetGroups = []CapabilityCodeSetGroup{
	{ID: CapabilityApplyment, Name: "特约商户进件与结算账户", Endpoints: []EndpointID{EndpointApplymentSubmit, EndpointApplymentQueryByID, EndpointApplymentQueryByBusinessCode, EndpointSettlementModify, EndpointSettlementQuery, EndpointSettlementModificationQuery, EndpointMerchantMediaUpload}},
	{ID: CapabilityAccountWillingness, Name: "商户开户意愿确认", Endpoints: []EndpointID{EndpointAccountWillingnessSubmit, EndpointAccountWillingnessCancel, EndpointAccountWillingnessQuery, EndpointAccountAuthorizeState, EndpointAccountWillingnessMediaUpload}},
	{ID: CapabilityMerchantManagement, Name: "商户管控、商户平台处置通知与不活跃核实", Endpoints: []EndpointID{EndpointViolationNotificationConfigQuery, EndpointViolationNotificationConfigUpdate, EndpointViolationNotificationConfigCreate, EndpointViolationNotificationConfigDelete, EndpointMerchantLimitationQuery, EndpointInactiveMerchantVerificationCreate, EndpointInactiveMerchantVerificationQuery}},
	{ID: CapabilityPayment, Name: "小程序支付", Endpoints: []EndpointID{EndpointPaymentPrepay, EndpointPaymentQueryByTransactionID, EndpointPaymentQueryByOutTradeNo, EndpointPaymentClose, EndpointPaymentRefundCreate, EndpointPaymentRefundQuery}},
	{ID: CapabilityCombinePayment, Name: "小程序合单支付", Endpoints: []EndpointID{EndpointCombinePrepay, EndpointCombineQuery, EndpointCombineClose, EndpointCombineRefundCreate, EndpointCombineRefundQuery}},
	{ID: CapabilityRefund, Name: "订单退款", Endpoints: []EndpointID{EndpointRefundCreate, EndpointRefundQuery}},
	{ID: CapabilityProfitSharing, Name: "分账", Endpoints: []EndpointID{EndpointProfitSharingCreate, EndpointProfitSharingQuery, EndpointProfitSharingReturnCreate, EndpointProfitSharingReturnQuery, EndpointProfitSharingUnfreeze, EndpointProfitSharingRemainingAmount, EndpointProfitSharingReceiverAdd, EndpointProfitSharingReceiverDelete}},
}

var endpointCodeSets = map[EndpointID]DocumentedCodeSet{
	EndpointApplymentSubmit:                    ApplymentSubmitDocumentedCodes,
	EndpointApplymentQueryByID:                 ApplymentQueryDocumentedCodes,
	EndpointApplymentQueryByBusinessCode:       ApplymentQueryDocumentedCodes,
	EndpointSettlementModify:                   SettlementModifyDocumentedCodes,
	EndpointSettlementQuery:                    SettlementQueryDocumentedCodes,
	EndpointSettlementModificationQuery:        SettlementModificationQueryDocumentedCodes,
	EndpointMerchantMediaUpload:                MerchantMediaUploadDocumentedCodes,
	EndpointAccountWillingnessSubmit:           AccountWillingnessSubmitDocumentedCodes,
	EndpointAccountWillingnessCancel:           AccountWillingnessCancelDocumentedCodes,
	EndpointAccountWillingnessQuery:            AccountWillingnessQueryDocumentedCodes,
	EndpointAccountAuthorizeState:              AccountAuthorizeStateDocumentedCodes,
	EndpointAccountWillingnessMediaUpload:      AccountWillingnessMediaUploadDocumentedCodes,
	EndpointViolationNotificationConfigQuery:   ViolationNotificationConfigQueryDocumentedCodes,
	EndpointViolationNotificationConfigUpdate:  ViolationNotificationConfigUpdateDocumentedCodes,
	EndpointViolationNotificationConfigCreate:  ViolationNotificationConfigCreateDocumentedCodes,
	EndpointViolationNotificationConfigDelete:  ViolationNotificationConfigDeleteDocumentedCodes,
	EndpointMerchantLimitationQuery:            MerchantLimitationQueryDocumentedCodes,
	EndpointInactiveMerchantVerificationCreate: InactiveMerchantIdentityVerificationCreateDocumentedCodes,
	EndpointInactiveMerchantVerificationQuery:  InactiveMerchantIdentityVerificationQueryDocumentedCodes,
	EndpointPaymentPrepay:                      PaymentPrepayDocumentedCodes,
	EndpointPaymentQueryByTransactionID:        PaymentQueryDocumentedCodes,
	EndpointPaymentQueryByOutTradeNo:           PaymentQueryDocumentedCodes,
	EndpointPaymentClose:                       PaymentCloseDocumentedCodes,
	EndpointPaymentRefundCreate:                RefundCreateDocumentedCodes,
	EndpointPaymentRefundQuery:                 RefundQueryDocumentedCodes,
	EndpointCombinePrepay:                      CombinePrepayDocumentedCodes,
	EndpointCombineQuery:                       CombineQueryDocumentedCodes,
	EndpointCombineClose:                       CombineCloseDocumentedCodes,
	EndpointCombineRefundCreate:                RefundCreateDocumentedCodes,
	EndpointCombineRefundQuery:                 RefundQueryDocumentedCodes,
	EndpointRefundCreate:                       RefundCreateDocumentedCodes,
	EndpointRefundQuery:                        RefundQueryDocumentedCodes,
	EndpointProfitSharingCreate:                ProfitSharingCreateDocumentedCodes,
	EndpointProfitSharingQuery:                 ProfitSharingQueryDocumentedCodes,
	EndpointProfitSharingReturnCreate:          ProfitSharingReturnCreateDocumentedCodes,
	EndpointProfitSharingReturnQuery:           ProfitSharingReturnQueryDocumentedCodes,
	EndpointProfitSharingUnfreeze:              ProfitSharingUnfreezeDocumentedCodes,
	EndpointProfitSharingRemainingAmount:       ProfitSharingRemainingAmountDocumentedCodes,
	EndpointProfitSharingReceiverAdd:           ProfitSharingReceiverAddDocumentedCodes,
	EndpointProfitSharingReceiverDelete:        ProfitSharingReceiverDeleteDocumentedCodes,
}

func CapabilityCodeSetGroups() []CapabilityCodeSetGroup {
	return append([]CapabilityCodeSetGroup(nil), capabilityCodeSetGroups...)
}

func EndpointCodeSets() map[EndpointID]DocumentedCodeSet {
	copied := make(map[EndpointID]DocumentedCodeSet, len(endpointCodeSets))
	for id, set := range endpointCodeSets {
		copied[id] = set
	}
	return copied
}

func EndpointCodeSetByID(id EndpointID) (DocumentedCodeSet, bool) {
	set, ok := endpointCodeSets[id]
	return set, ok
}
