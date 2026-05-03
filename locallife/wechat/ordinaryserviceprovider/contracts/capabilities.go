package contracts

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

type CapabilityID string

const (
	CapabilityApplyment          CapabilityID = "applyment"
	CapabilityMerchantManagement CapabilityID = "merchant_management"
	CapabilityPayment            CapabilityID = "payment"
	CapabilityCombinePayment     CapabilityID = "combine_payment"
	CapabilityRefund             CapabilityID = "refund"
	CapabilityProfitSharing      CapabilityID = "profit_sharing"
)

type EndpointID string

const (
	EndpointApplymentSubmit                    EndpointID = "applyment.submit"
	EndpointApplymentQueryByID                 EndpointID = "applyment.query_by_id"
	EndpointApplymentQueryByBusinessCode       EndpointID = "applyment.query_by_business_code"
	EndpointSettlementModify                   EndpointID = "applyment.settlement_modify"
	EndpointSettlementQuery                    EndpointID = "applyment.settlement_query"
	EndpointSettlementModificationQuery        EndpointID = "applyment.settlement_modification_query"
	EndpointMerchantMediaUpload                EndpointID = "applyment.media_upload"
	EndpointViolationNotificationConfigQuery   EndpointID = "merchant_management.violation_notification_config_query"
	EndpointViolationNotificationConfigUpdate  EndpointID = "merchant_management.violation_notification_config_update"
	EndpointViolationNotificationConfigCreate  EndpointID = "merchant_management.violation_notification_config_create"
	EndpointViolationNotificationConfigDelete  EndpointID = "merchant_management.violation_notification_config_delete"
	EndpointMerchantViolationNotification      EndpointID = "merchant_management.violation_notification"
	EndpointMerchantLimitationQuery            EndpointID = "merchant_management.limitation_query"
	EndpointInactiveMerchantVerificationCreate EndpointID = "merchant_management.inactive_verification_create"
	EndpointInactiveMerchantVerificationQuery  EndpointID = "merchant_management.inactive_verification_query"
	EndpointPaymentPrepay                      EndpointID = "payment.prepay"
	EndpointPaymentNotify                      EndpointID = "payment.notify"
	EndpointPaymentQueryByTransactionID        EndpointID = "payment.query_by_transaction_id"
	EndpointPaymentQueryByOutTradeNo           EndpointID = "payment.query_by_out_trade_no"
	EndpointPaymentClose                       EndpointID = "payment.close"
	EndpointPaymentRefundCreate                EndpointID = "payment.refund_create"
	EndpointPaymentRefundQuery                 EndpointID = "payment.refund_query"
	EndpointPaymentRefundNotify                EndpointID = "payment.refund_notify"
	EndpointCombinePrepay                      EndpointID = "combine_payment.prepay"
	EndpointCombineJSAPIPayParams              EndpointID = "combine_payment.jsapi_pay_params"
	EndpointCombineQuery                       EndpointID = "combine_payment.query"
	EndpointCombineClose                       EndpointID = "combine_payment.close"
	EndpointCombineNotify                      EndpointID = "combine_payment.notify"
	EndpointCombineRefundCreate                EndpointID = "combine_payment.refund_create"
	EndpointCombineRefundQuery                 EndpointID = "combine_payment.refund_query"
	EndpointCombineRefundNotify                EndpointID = "combine_payment.refund_notify"
	EndpointRefundCreate                       EndpointID = "refund.create"
	EndpointRefundQuery                        EndpointID = "refund.query"
	EndpointRefundNotify                       EndpointID = "refund.notify"
	EndpointProfitSharingCreate                EndpointID = "profit_sharing.create"
	EndpointProfitSharingQuery                 EndpointID = "profit_sharing.query"
	EndpointProfitSharingReturnCreate          EndpointID = "profit_sharing.return_create"
	EndpointProfitSharingReturnQuery           EndpointID = "profit_sharing.return_query"
	EndpointProfitSharingUnfreeze              EndpointID = "profit_sharing.unfreeze"
	EndpointProfitSharingRemainingAmount       EndpointID = "profit_sharing.remaining_amount"
	EndpointProfitSharingReceiverAdd           EndpointID = "profit_sharing.receiver_add"
	EndpointProfitSharingReceiverDelete        EndpointID = "profit_sharing.receiver_delete"
	EndpointProfitSharingNotify                EndpointID = "profit_sharing.notify"
)

type RequestValidator func(any) error

type EndpointContract struct {
	ID               EndpointID
	Capability       CapabilityID
	Operation        string
	Method           string
	Path             string
	RequestTypes     []reflect.Type
	ResponseTypes    []reflect.Type
	StatusOwners     []string
	RequestValidator RequestValidator
}

type CapabilityGroup struct {
	ID        CapabilityID
	Name      string
	Endpoints []EndpointID
}

var capabilityGroups = []CapabilityGroup{
	{ID: CapabilityApplyment, Name: "特约商户进件与结算账户", Endpoints: []EndpointID{EndpointApplymentSubmit, EndpointApplymentQueryByID, EndpointApplymentQueryByBusinessCode, EndpointSettlementModify, EndpointSettlementQuery, EndpointSettlementModificationQuery, EndpointMerchantMediaUpload}},
	{ID: CapabilityMerchantManagement, Name: "商户管控、商户平台处置通知与不活跃核实", Endpoints: []EndpointID{EndpointViolationNotificationConfigQuery, EndpointViolationNotificationConfigUpdate, EndpointViolationNotificationConfigCreate, EndpointViolationNotificationConfigDelete, EndpointMerchantViolationNotification, EndpointMerchantLimitationQuery, EndpointInactiveMerchantVerificationCreate, EndpointInactiveMerchantVerificationQuery}},
	{ID: CapabilityPayment, Name: "小程序支付", Endpoints: []EndpointID{EndpointPaymentPrepay, EndpointPaymentNotify, EndpointPaymentQueryByTransactionID, EndpointPaymentQueryByOutTradeNo, EndpointPaymentClose, EndpointPaymentRefundCreate, EndpointPaymentRefundQuery, EndpointPaymentRefundNotify}},
	{ID: CapabilityCombinePayment, Name: "小程序合单支付", Endpoints: []EndpointID{EndpointCombinePrepay, EndpointCombineJSAPIPayParams, EndpointCombineQuery, EndpointCombineClose, EndpointCombineNotify, EndpointCombineRefundCreate, EndpointCombineRefundQuery, EndpointCombineRefundNotify}},
	{ID: CapabilityRefund, Name: "订单退款", Endpoints: []EndpointID{EndpointRefundCreate, EndpointRefundQuery, EndpointRefundNotify}},
	{ID: CapabilityProfitSharing, Name: "分账", Endpoints: []EndpointID{EndpointProfitSharingCreate, EndpointProfitSharingQuery, EndpointProfitSharingReturnCreate, EndpointProfitSharingReturnQuery, EndpointProfitSharingUnfreeze, EndpointProfitSharingRemainingAmount, EndpointProfitSharingReceiverAdd, EndpointProfitSharingReceiverDelete, EndpointProfitSharingNotify}},
}

var endpointContracts = map[EndpointID]EndpointContract{
	EndpointApplymentSubmit:                    endpoint(EndpointApplymentSubmit, CapabilityApplyment, "submit ordinary service provider applyment", http.MethodPost, "/v3/applyment4sub/applyment/", []any{ApplymentSubmitRequest{}}, []any{ApplymentSubmitResponse{}}, nil, validateTyped(func(r ApplymentSubmitRequest) error { return r.Validate() })),
	EndpointApplymentQueryByID:                 endpoint(EndpointApplymentQueryByID, CapabilityApplyment, "query ordinary service provider applyment by id", http.MethodGet, "/v3/applyment4sub/applyment/applyment_id/{applyment_id}", []any{ApplymentQueryByIDRequest{}}, []any{ApplymentQueryResponse{}}, []string{"ApplymentState"}, validateTyped(validateApplymentQueryByIDRequest)),
	EndpointApplymentQueryByBusinessCode:       endpoint(EndpointApplymentQueryByBusinessCode, CapabilityApplyment, "query ordinary service provider applyment by business code", http.MethodGet, "/v3/applyment4sub/applyment/business_code/{business_code}", []any{ApplymentQueryByBusinessCodeRequest{}}, []any{ApplymentQueryResponse{}}, []string{"ApplymentState"}, validateTyped(validateApplymentQueryByBusinessCodeRequest)),
	EndpointSettlementModify:                   endpoint(EndpointSettlementModify, CapabilityApplyment, "modify ordinary service provider settlement", http.MethodPost, "/v3/apply4sub/sub_merchants/{sub_mchid}/modify-settlement", []any{SettlementModifyRequest{}}, []any{SettlementModifyResponse{}}, nil, validateTyped(validateSettlementModifyRequest)),
	EndpointSettlementQuery:                    endpoint(EndpointSettlementQuery, CapabilityApplyment, "query ordinary service provider settlement", http.MethodGet, "/v3/apply4sub/sub_merchants/{sub_mchid}/settlement", []any{SettlementQueryRequest{}}, []any{SettlementQueryResponse{}}, []string{"SettlementVerifyResult"}, validateTyped(validateSettlementQueryRequest)),
	EndpointSettlementModificationQuery:        endpoint(EndpointSettlementModificationQuery, CapabilityApplyment, "query ordinary service provider settlement modification", http.MethodGet, "/v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}", []any{SettlementModificationQueryRequest{}}, []any{SettlementModificationQueryResponse{}}, []string{"SettlementVerifyResult", "SettlementAuditResult"}, validateTyped(validateSettlementModificationQueryRequest)),
	EndpointMerchantMediaUpload:                endpoint(EndpointMerchantMediaUpload, CapabilityApplyment, "upload ordinary service provider applyment image", http.MethodPost, "/v3/merchant/media/upload", []any{MediaUploadRequestMultipart{}}, []any{MediaUploadResponse{}}, nil, validateTyped(validateMediaUploadRequest)),
	EndpointViolationNotificationConfigQuery:   endpoint(EndpointViolationNotificationConfigQuery, CapabilityMerchantManagement, "query ordinary service provider violation notification config", http.MethodGet, "/v3/merchant-risk-manage/violation-notifications", []any{NoRequestBody{}}, []any{ViolationNotificationConfigResponse{}}, nil, validateNoRequestBody),
	EndpointViolationNotificationConfigUpdate:  endpoint(EndpointViolationNotificationConfigUpdate, CapabilityMerchantManagement, "update ordinary service provider violation notification config", http.MethodPut, "/v3/merchant-risk-manage/violation-notifications", []any{ViolationNotificationConfigRequest{}}, []any{ViolationNotificationConfigResponse{}}, nil, validateTyped(validateViolationNotificationConfigRequest)),
	EndpointViolationNotificationConfigCreate:  endpoint(EndpointViolationNotificationConfigCreate, CapabilityMerchantManagement, "create ordinary service provider violation notification config", http.MethodPost, "/v3/merchant-risk-manage/violation-notifications", []any{ViolationNotificationConfigRequest{}}, []any{ViolationNotificationConfigResponse{}}, nil, validateTyped(validateViolationNotificationConfigRequest)),
	EndpointViolationNotificationConfigDelete:  endpoint(EndpointViolationNotificationConfigDelete, CapabilityMerchantManagement, "delete ordinary service provider violation notification config", http.MethodDelete, "/v3/merchant-risk-manage/violation-notifications", []any{NoRequestBody{}}, []any{NoResponseBody{}}, nil, validateNoRequestBody),
	EndpointMerchantViolationNotification:      endpoint(EndpointMerchantViolationNotification, CapabilityMerchantManagement, "ordinary service provider merchant violation notification", "CALLBACK", "configured merchant violation notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{MerchantViolationNotificationPayload{}, WechatErrorResponse{}}, nil, validateNotificationRequest),
	EndpointMerchantLimitationQuery:            endpoint(EndpointMerchantLimitationQuery, CapabilityMerchantManagement, "query ordinary service provider merchant limitation", http.MethodGet, "/v3/mch-operation-manage/merchant-limitations/sub-mchid/{sub_mchid}", []any{MerchantLimitationQueryRequest{}}, []any{MerchantLimitationQueryResponse{}}, []string{"MerchantLimitedFunction"}, validateTyped(validateMerchantLimitationQueryRequest)),
	EndpointInactiveMerchantVerificationCreate: endpoint(EndpointInactiveMerchantVerificationCreate, CapabilityMerchantManagement, "create ordinary service provider inactive merchant identity verification", http.MethodPost, "/v3/compliance/inactive-merchant-identity-verification/merchants", []any{InactiveMerchantIdentityVerificationCreateRequest{}}, []any{InactiveMerchantIdentityVerificationCreateResponse{}}, nil, validateTyped(validateInactiveMerchantIdentityVerificationCreateRequest)),
	EndpointInactiveMerchantVerificationQuery:  endpoint(EndpointInactiveMerchantVerificationQuery, CapabilityMerchantManagement, "query ordinary service provider inactive merchant identity verification", http.MethodGet, "/v3/compliance/inactive-merchant-identity-verification/merchants/{sub_mchid}/verifications/{verification_id}", []any{InactiveMerchantIdentityVerificationQueryRequest{}}, []any{InactiveMerchantIdentityVerificationQueryResponse{}}, []string{"InactiveMerchantIdentityVerificationState"}, validateTyped(validateInactiveMerchantIdentityVerificationQueryRequest)),
	EndpointPaymentPrepay:                      endpoint(EndpointPaymentPrepay, CapabilityPayment, "create ordinary service provider payment", http.MethodPost, "/v3/pay/partner/transactions/jsapi", []any{PaymentPrepayRequest{}}, []any{PaymentPrepayResponse{}}, nil, validateTyped(func(r PaymentPrepayRequest) error { return r.Validate() })),
	EndpointPaymentNotify:                      endpoint(EndpointPaymentNotify, CapabilityPayment, "ordinary service provider payment notification", "CALLBACK", "configured payment notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{PaymentNotificationPayload{}, WechatErrorResponse{}}, []string{"PaymentTradeState"}, validateNotificationRequest),
	EndpointPaymentQueryByTransactionID:        endpoint(EndpointPaymentQueryByTransactionID, CapabilityPayment, "query ordinary service provider payment by transaction id", http.MethodGet, "/v3/pay/partner/transactions/id/{transaction_id}", []any{PaymentQueryRequest{}}, []any{PaymentQueryResponse{}}, []string{"PaymentTradeState"}, validateTyped(validatePaymentQueryByTransactionIDRequest)),
	EndpointPaymentQueryByOutTradeNo:           endpoint(EndpointPaymentQueryByOutTradeNo, CapabilityPayment, "query ordinary service provider payment by out trade no", http.MethodGet, "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}", []any{PaymentQueryRequest{}}, []any{PaymentQueryResponse{}}, []string{"PaymentTradeState"}, validateTyped(validatePaymentQueryByOutTradeNoRequest)),
	EndpointPaymentClose:                       endpoint(EndpointPaymentClose, CapabilityPayment, "close ordinary service provider payment", http.MethodPost, "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close", []any{PaymentCloseRequest{}}, []any{NoResponseBody{}}, nil, validateTyped(validatePaymentCloseRequest)),
	EndpointPaymentRefundCreate:                endpoint(EndpointPaymentRefundCreate, CapabilityPayment, "create ordinary service provider payment refund", http.MethodPost, "/v3/refund/domestic/refunds", []any{RefundCreateRequest{}}, []any{RefundResponse{}}, []string{"RefundStatus"}, validateTyped(func(r RefundCreateRequest) error { return r.Validate() })),
	EndpointPaymentRefundQuery:                 endpoint(EndpointPaymentRefundQuery, CapabilityPayment, "query ordinary service provider payment refund", http.MethodGet, "/v3/refund/domestic/refunds/{out_refund_no}", []any{RefundQueryRequest{}}, []any{RefundResponse{}}, []string{"RefundStatus"}, validateTyped(validateRefundQueryRequest)),
	EndpointPaymentRefundNotify:                endpoint(EndpointPaymentRefundNotify, CapabilityPayment, "ordinary service provider refund notification", "CALLBACK", "configured refund notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{RefundNotificationPayload{}, WechatErrorResponse{}}, []string{"RefundStatus"}, validateNotificationRequest),
	EndpointCombinePrepay:                      endpoint(EndpointCombinePrepay, CapabilityCombinePayment, "create ordinary service provider combine payment", http.MethodPost, "/v3/combine-transactions/jsapi", []any{CombinePrepayRequest{}}, []any{CombinePrepayResponse{}}, nil, validateTyped(func(r CombinePrepayRequest) error { return r.Validate() })),
	EndpointCombineJSAPIPayParams:              endpoint(EndpointCombineJSAPIPayParams, CapabilityCombinePayment, "ordinary service provider combine jsapi pay params", "LOCAL", "wx.requestPayment parameters", nil, []any{JSAPIPayParams{}}, nil, nil),
	EndpointCombineQuery:                       endpoint(EndpointCombineQuery, CapabilityCombinePayment, "query ordinary service provider combine payment", http.MethodGet, "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}", []any{CombineQueryRequest{}}, []any{CombineQueryResponse{}}, []string{"PaymentTradeState"}, validateTyped(validateCombineQueryRequest)),
	EndpointCombineClose:                       endpoint(EndpointCombineClose, CapabilityCombinePayment, "close ordinary service provider combine payment", http.MethodPost, "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close", []any{CombineCloseRequest{}}, []any{NoResponseBody{}}, nil, validateTyped(validateCombineCloseRequest)),
	EndpointCombineNotify:                      endpoint(EndpointCombineNotify, CapabilityCombinePayment, "ordinary service provider combine payment notification", "CALLBACK", "configured combine payment notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{CombinePaymentNotificationPayload{}, WechatErrorResponse{}}, []string{"PaymentTradeState"}, validateNotificationRequest),
	EndpointCombineRefundCreate:                endpoint(EndpointCombineRefundCreate, CapabilityCombinePayment, "create ordinary service provider combine refund", http.MethodPost, "/v3/refund/domestic/refunds", []any{RefundCreateRequest{}}, []any{RefundResponse{}}, []string{"RefundStatus"}, validateTyped(func(r RefundCreateRequest) error { return r.Validate() })),
	EndpointCombineRefundQuery:                 endpoint(EndpointCombineRefundQuery, CapabilityCombinePayment, "query ordinary service provider combine refund", http.MethodGet, "/v3/refund/domestic/refunds/{out_refund_no}", []any{RefundQueryRequest{}}, []any{RefundResponse{}}, []string{"RefundStatus"}, validateTyped(validateRefundQueryRequest)),
	EndpointCombineRefundNotify:                endpoint(EndpointCombineRefundNotify, CapabilityCombinePayment, "ordinary service provider combine refund notification", "CALLBACK", "configured refund notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{RefundNotificationPayload{}, WechatErrorResponse{}}, []string{"RefundStatus"}, validateNotificationRequest),
	EndpointRefundCreate:                       endpoint(EndpointRefundCreate, CapabilityRefund, "create ordinary service provider refund", http.MethodPost, "/v3/refund/domestic/refunds", []any{RefundCreateRequest{}}, []any{RefundResponse{}}, []string{"RefundStatus"}, validateTyped(func(r RefundCreateRequest) error { return r.Validate() })),
	EndpointRefundQuery:                        endpoint(EndpointRefundQuery, CapabilityRefund, "query ordinary service provider refund", http.MethodGet, "/v3/refund/domestic/refunds/{out_refund_no}", []any{RefundQueryRequest{}}, []any{RefundResponse{}}, []string{"RefundStatus"}, validateTyped(validateRefundQueryRequest)),
	EndpointRefundNotify:                       endpoint(EndpointRefundNotify, CapabilityRefund, "ordinary service provider refund notification", "CALLBACK", "configured refund notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{RefundNotificationPayload{}, WechatErrorResponse{}}, []string{"RefundStatus"}, validateNotificationRequest),
	EndpointProfitSharingCreate:                endpoint(EndpointProfitSharingCreate, CapabilityProfitSharing, "create ordinary service provider profit sharing order", http.MethodPost, "/v3/profitsharing/orders", []any{ProfitSharingOrderRequest{}}, []any{ProfitSharingOrderResponse{}}, []string{"ProfitSharingOrderState", "ProfitSharingReceiverResult", "ProfitSharingFailReason"}, validateTyped(func(r ProfitSharingOrderRequest) error { return r.Validate() })),
	EndpointProfitSharingQuery:                 endpoint(EndpointProfitSharingQuery, CapabilityProfitSharing, "query ordinary service provider profit sharing order", http.MethodGet, "/v3/profitsharing/orders/{out_order_no}", []any{ProfitSharingQueryRequest{}}, []any{ProfitSharingOrderResponse{}}, []string{"ProfitSharingOrderState", "ProfitSharingReceiverResult", "ProfitSharingFailReason"}, validateTyped(validateProfitSharingQueryRequest)),
	EndpointProfitSharingReturnCreate:          endpoint(EndpointProfitSharingReturnCreate, CapabilityProfitSharing, "create ordinary service provider profit sharing return", http.MethodPost, "/v3/profitsharing/return-orders", []any{ProfitSharingReturnRequest{}}, []any{ProfitSharingReturnResponse{}}, []string{"ProfitSharingReturnState"}, validateTyped(validateProfitSharingReturnRequest)),
	EndpointProfitSharingReturnQuery:           endpoint(EndpointProfitSharingReturnQuery, CapabilityProfitSharing, "query ordinary service provider profit sharing return", http.MethodGet, "/v3/profitsharing/return-orders/{out_return_no}", []any{ProfitSharingReturnQueryRequest{}}, []any{ProfitSharingReturnResponse{}}, []string{"ProfitSharingReturnState"}, validateTyped(validateProfitSharingReturnQueryRequest)),
	EndpointProfitSharingUnfreeze:              endpoint(EndpointProfitSharingUnfreeze, CapabilityProfitSharing, "unfreeze ordinary service provider profit sharing order", http.MethodPost, "/v3/profitsharing/orders/unfreeze", []any{ProfitSharingUnfreezeRequest{}}, []any{ProfitSharingUnfreezeResponse{}}, []string{"ProfitSharingOrderState"}, validateTyped(validateProfitSharingUnfreezeRequest)),
	EndpointProfitSharingRemainingAmount:       endpoint(EndpointProfitSharingRemainingAmount, CapabilityProfitSharing, "query ordinary service provider profit sharing remaining amount", http.MethodGet, "/v3/profitsharing/transactions/{transaction_id}/amounts", []any{ProfitSharingRemainingAmountRequest{}}, []any{ProfitSharingRemainingAmountResponse{}}, nil, validateTyped(validateProfitSharingRemainingAmountRequest)),
	EndpointProfitSharingReceiverAdd:           endpoint(EndpointProfitSharingReceiverAdd, CapabilityProfitSharing, "add ordinary service provider profit sharing receiver", http.MethodPost, "/v3/profitsharing/receivers/add", []any{ProfitSharingReceiverAddRequest{}}, []any{ProfitSharingReceiverResponse{}}, []string{"ReceiverType", "ProfitSharingReceiverRelationType"}, validateTyped(validateProfitSharingReceiverAddRequest)),
	EndpointProfitSharingReceiverDelete:        endpoint(EndpointProfitSharingReceiverDelete, CapabilityProfitSharing, "delete ordinary service provider profit sharing receiver", http.MethodPost, "/v3/profitsharing/receivers/delete", []any{ProfitSharingReceiverDeleteRequest{}}, []any{ProfitSharingReceiverResponse{}}, []string{"ReceiverType"}, validateTyped(validateProfitSharingReceiverDeleteRequest)),
	EndpointProfitSharingNotify:                endpoint(EndpointProfitSharingNotify, CapabilityProfitSharing, "ordinary service provider profit sharing notification", "CALLBACK", "configured profit sharing notify_url", []any{NotificationRequest{}, NotificationResource{}}, []any{ProfitSharingNotificationPayload{}, WechatErrorResponse{}}, []string{"ProfitSharingReceiverResult"}, validateNotificationRequest),
}

func CapabilityGroups() []CapabilityGroup {
	return append([]CapabilityGroup(nil), capabilityGroups...)
}

func EndpointContracts() map[EndpointID]EndpointContract {
	copied := make(map[EndpointID]EndpointContract, len(endpointContracts))
	for id, contract := range endpointContracts {
		copied[id] = contract
	}
	return copied
}

func EndpointContractByID(id EndpointID) (EndpointContract, bool) {
	contract, ok := endpointContracts[id]
	return contract, ok
}

func ValidateEndpointRequest(id EndpointID, request any) error {
	contract, ok := EndpointContractByID(id)
	if !ok {
		return ValidationError{Field: "endpoint", Code: "unknown_endpoint", Message: fmt.Sprintf("unknown ordinary service provider endpoint %s", id)}
	}
	if len(contract.RequestTypes) == 0 {
		if request == nil {
			return nil
		}
		return ValidationError{Field: "request", Code: "unexpected_request", Message: fmt.Sprintf("%s does not accept a request body", id)}
	}
	if contract.RequestValidator == nil {
		return nil
	}
	return contract.RequestValidator(request)
}

func endpoint(id EndpointID, capability CapabilityID, operation, method, path string, requestTypes []any, responseTypes []any, statusOwners []string, validator RequestValidator) EndpointContract {
	return EndpointContract{ID: id, Capability: capability, Operation: operation, Method: method, Path: path, RequestTypes: contractTypes(requestTypes...), ResponseTypes: contractTypes(responseTypes...), StatusOwners: append([]string(nil), statusOwners...), RequestValidator: validator}
}

func contractTypes(values ...any) []reflect.Type {
	if len(values) == 0 {
		return nil
	}
	types := make([]reflect.Type, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		typ := reflect.TypeOf(value)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		types = append(types, typ)
	}
	return types
}

func validateTyped[T any](fn func(T) error) RequestValidator {
	return func(value any) error {
		if request, ok := value.(T); ok {
			return fn(request)
		}
		if request, ok := value.(*T); ok && request != nil {
			return fn(*request)
		}
		var zero T
		return ValidationError{Field: "request", Code: "invalid_type", Message: fmt.Sprintf("request must be %T", zero)}
	}
}

func validateNoRequestBody(value any) error {
	if value == nil {
		return nil
	}
	if _, ok := value.(NoRequestBody); ok {
		return nil
	}
	return ValidationError{Field: "request", Code: "unexpected_request", Message: "endpoint must not receive a request body"}
}

func validateNotificationRequest(value any) error {
	request, ok := value.(NotificationRequest)
	if !ok {
		if ptr, ptrOK := value.(*NotificationRequest); ptrOK && ptr != nil {
			request = *ptr
			ok = true
		}
	}
	if !ok {
		return ValidationError{Field: "request", Code: "invalid_type", Message: "request must be NotificationRequest"}
	}
	for _, field := range []struct{ name, value string }{{"id", request.ID}, {"event_type", request.EventType}, {"resource_type", request.ResourceType}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if request.Resource == nil {
		return missing("resource")
	}
	return nil
}

func validateApplymentQueryByIDRequest(r ApplymentQueryByIDRequest) error {
	return requirePositiveInt("applyment_id", r.ApplymentID)
}

func validateApplymentQueryByBusinessCodeRequest(r ApplymentQueryByBusinessCodeRequest) error {
	return requireString("business_code", r.BusinessCode)
}

func validateSettlementQueryRequest(r SettlementQueryRequest) error {
	return requireString("sub_mchid", r.SubMchID)
}

func validateSettlementModifyRequest(r SettlementModifyRequest) error {
	for _, field := range []struct{ name, value string }{{"sub_mchid", r.SubMchID}, {"account_type", string(r.AccountType)}, {"account_bank", r.AccountBank}, {"account_number", r.AccountNumber}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateSettlementModificationQueryRequest(r SettlementModificationQueryRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	if strings.TrimSpace(r.ApplicationNo) == "" && strings.TrimSpace(r.ApplicationID) == "" {
		return missing("application_no")
	}
	return nil
}

func validateMediaUploadRequest(r MediaUploadRequestMultipart) error {
	if len(r.File) == 0 {
		return missing("file")
	}
	if err := requireString("meta.filename", r.Meta.Filename); err != nil {
		return err
	}
	return requireString("meta.sha256", r.Meta.SHA256)
}

func validateMerchantLimitationQueryRequest(r MerchantLimitationQueryRequest) error {
	return requireString("sub_mchid", r.SubMchID)
}

func validateViolationNotificationConfigRequest(r ViolationNotificationConfigRequest) error {
	return validateHTTPSURL("notify_url", r.NotifyURL, true)
}

func validateInactiveMerchantIdentityVerificationCreateRequest(r InactiveMerchantIdentityVerificationCreateRequest) error {
	return requireString("sub_mchid", r.SubMchID)
}

func validateInactiveMerchantIdentityVerificationQueryRequest(r InactiveMerchantIdentityVerificationQueryRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	if strings.TrimSpace(r.VerificationID) == "" && strings.TrimSpace(r.VerifyID) == "" {
		return missing("verification_id")
	}
	return nil
}

func validatePaymentQueryByTransactionIDRequest(r PaymentQueryRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	return requireString("transaction_id", r.TransactionID)
}

func validatePaymentQueryByOutTradeNoRequest(r PaymentQueryRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	return requireString("out_trade_no", r.OutTradeNo)
}

func validatePaymentCloseRequest(r PaymentCloseRequest) error {
	for _, field := range []struct{ name, value string }{{"sp_mchid", r.SpMchID}, {"sub_mchid", r.SubMchID}, {"out_trade_no", r.OutTradeNo}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateCombineQueryRequest(r CombineQueryRequest) error {
	return requireString("combine_out_trade_no", r.CombineOutTradeNo)
}

func validateCombineCloseRequest(r CombineCloseRequest) error {
	if err := requireString("combine_appid", r.CombineAppID); err != nil {
		return err
	}
	if err := requireString("combine_out_trade_no", r.CombineOutTradeNo); err != nil {
		return err
	}
	if len(r.SubOrders) == 0 {
		return missing("sub_orders")
	}
	for index, order := range r.SubOrders {
		prefix := fmt.Sprintf("sub_orders[%d]", index)
		for _, field := range []struct{ name, value string }{{prefix + ".mchid", order.MchID}, {prefix + ".out_trade_no", order.OutTradeNo}, {prefix + ".sub_mchid", order.SubMchID}} {
			if err := requireString(field.name, field.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRefundQueryRequest(r RefundQueryRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	return requireString("out_refund_no", r.OutRefundNo)
}

func validateProfitSharingReceiverAddRequest(r ProfitSharingReceiverAddRequest) error {
	if err := validateProfitSharingReceiverFields(r.SubMchID, r.AppID, r.SubAppID, r.Type, r.Account); err != nil {
		return err
	}
	if err := requireMerchantReceiverName(r.Type, r.Name); err != nil {
		return err
	}
	if r.RelationType == "" {
		return missing("relation_type")
	}
	if !validProfitSharingRelationType(r.RelationType) {
		return invalidEnum("relation_type")
	}
	if r.RelationType == ProfitSharingRelationCustom && strings.TrimSpace(r.CustomRelation) == "" {
		return missing("custom_relation")
	}
	return nil
}

func validateProfitSharingReceiverDeleteRequest(r ProfitSharingReceiverDeleteRequest) error {
	return validateProfitSharingReceiverFields(r.SubMchID, r.AppID, r.SubAppID, r.Type, r.Account)
}

func validateProfitSharingReceiverFields(subMchID, appID, subAppID string, receiverType ReceiverType, account string) error {
	for _, field := range []struct{ name, value string }{{"sub_mchid", subMchID}, {"appid", appID}, {"type", string(receiverType)}, {"account", account}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if !validReceiverType(receiverType) {
		return invalidEnum("type")
	}
	if receiverType == ReceiverTypePersonalSubOpenID && strings.TrimSpace(subAppID) == "" {
		return missing("sub_appid")
	}
	return nil
}

func requireMerchantReceiverName(receiverType ReceiverType, name string) error {
	if receiverType == ReceiverTypeMerchantID {
		return requireString("name", name)
	}
	return nil
}

func validateProfitSharingQueryRequest(r ProfitSharingQueryRequest) error {
	for _, field := range []struct{ name, value string }{{"sub_mchid", r.SubMchID}, {"transaction_id", r.TransactionID}, {"out_order_no", r.OutOrderNo}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateProfitSharingReturnRequest(r ProfitSharingReturnRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	if strings.TrimSpace(r.OutOrderNo) == "" && strings.TrimSpace(r.OrderID) == "" {
		return missing("out_order_no_or_order_id")
	}
	for _, field := range []struct{ name, value string }{{"out_return_no", r.OutReturnNo}, {"return_mchid", r.ReturnMchID}, {"description", r.Description}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return requirePositiveInt("amount", r.Amount)
}

func validateProfitSharingReturnQueryRequest(r ProfitSharingReturnQueryRequest) error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	if err := requireString("out_order_no", r.OutOrderNo); err != nil {
		return err
	}
	return requireString("out_return_no", r.OutReturnNo)
}

func validateProfitSharingUnfreezeRequest(r ProfitSharingUnfreezeRequest) error {
	for _, field := range []struct{ name, value string }{{"sub_mchid", r.SubMchID}, {"transaction_id", r.TransactionID}, {"out_order_no", r.OutOrderNo}, {"description", r.Description}} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateProfitSharingRemainingAmountRequest(r ProfitSharingRemainingAmountRequest) error {
	return requireString("transaction_id", r.TransactionID)
}
