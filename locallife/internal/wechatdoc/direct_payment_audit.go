package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type directPaymentEndpointContract struct {
	Method              string
	Path                string
	RequestFields       map[string]struct{}
	ResponseFields      map[string]struct{}
	RequestConstraints  map[string]map[string]struct{}
	ResponseConstraints map[string]map[string]struct{}
	RequestEnums        map[string]map[string]struct{}
	ResponseEnums       map[string]map[string]struct{}
	ErrorCodes          map[string]struct{}
}

func AuditDirectPaymentAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalDirectPaymentCode)
	contractInventory := directPaymentContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "direct_payment"}

	for _, key := range keys {
		doc := docEndpoints[key]
		if doc == nil {
			continue
		}
		report.Summary.DocumentedEndpointCount++
		endpointAudit := EndpointAlignmentAudit{Method: doc.Method, Path: doc.Path}
		contract, ok := contractInventory[key]
		if !ok {
			endpointAudit.MissingEndpoint = true
			endpointAudit.MissingRequestFields = sortedFieldNames(doc.RequestFields)
			endpointAudit.MissingResponseFields = sortedFieldNames(doc.ResponseFields)
			endpointAudit.MissingErrorCodes = sortedSetKeys(doc.ErrorCodes)
			report.Endpoints = append(report.Endpoints, endpointAudit)
			accumulateAlignmentSummary(&report.Summary, endpointAudit)
			continue
		}
		report.Summary.AuditedEndpointCount++
		endpointAudit.MissingRequestFields = diffFieldNames(doc.RequestFields, contract.RequestFields)
		endpointAudit.MissingResponseFields = diffFieldNames(doc.ResponseFields, contract.ResponseFields)
		endpointAudit.MissingRequestConstraints = diffFieldConstraints(doc.RequestFields, contract.RequestConstraints)
		endpointAudit.MissingResponseConstraints = diffFieldConstraints(doc.ResponseFields, contract.ResponseConstraints)
		endpointAudit.MissingRequestEnums = diffFieldEnums(doc.RequestFields, contract.RequestEnums, nil).Missing
		endpointAudit.MissingResponseEnums = diffFieldEnums(doc.ResponseFields, contract.ResponseEnums, nil).Missing
		endpointAudit.MissingErrorCodes = diffSet(doc.ErrorCodes, contract.ErrorCodes)
		if endpointHasMissingCoverage(endpointAudit) {
			report.Endpoints = append(report.Endpoints, endpointAudit)
			accumulateAlignmentSummary(&report.Summary, endpointAudit)
		}
	}

	return report
}

func directPaymentContractInventory() map[string]*directPaymentEndpointContract {
	createOrderRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectJSAPIOrderRequestBody{}))
	createOrderResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectJSAPIOrderResponse{}))
	queryOrderByTransactionRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectQueryOrderByTransactionIDRequest{}))
	queryOrderByOutTradeNoRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectQueryOrderByOutTradeNoRequest{}))
	queryOrderResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectOrderQueryResponse{}))
	paymentNotifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectPaymentNotificationResource{}))
	closeOrderRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectCloseOrderRequest{}))
	createRefundRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectRefundRequest{}))
	queryRefundRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectQueryRefundByOutRefundNoRequest{}))
	abnormalRefundRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectAbnormalRefundRequest{}))
	refundResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectRefundResponse{}))
	refundNotifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectRefundNotificationResource{}))

	closeOrderRequestFields["out_trade_no"] = struct{}{}

	booleanValues := setOf("true", "false")
	cnyValues := setOf(wechatcontracts.DirectPaymentCurrencyCNY)
	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	tradeTypeValues := setOf(
		wechatcontracts.DirectTradeTypeJSAPI,
		wechatcontracts.DirectTradeTypeNative,
		wechatcontracts.DirectTradeTypeApp,
		wechatcontracts.DirectTradeTypeMicropay,
		wechatcontracts.DirectTradeTypeMWEB,
		wechatcontracts.DirectTradeTypeFacePay,
	)
	tradeStateValues := setOf(
		wechatcontracts.DirectTradeStateSuccess,
		wechatcontracts.DirectTradeStateRefund,
		wechatcontracts.DirectTradeStateNotPay,
		wechatcontracts.DirectTradeStateClosed,
		wechatcontracts.DirectTradeStateRevoked,
		wechatcontracts.DirectTradeStateUserPaying,
		wechatcontracts.DirectTradeStatePayError,
	)
	promotionScopeValues := setOf(
		wechatcontracts.DirectPromotionScopeGlobal,
		wechatcontracts.DirectPromotionScopeSingle,
	)
	promotionTypeValues := setOf(
		wechatcontracts.DirectPromotionTypeCash,
		wechatcontracts.DirectPromotionTypeNoCash,
	)
	refundChannelValues := setOf(
		wechatcontracts.DirectRefundChannelOriginal,
		wechatcontracts.DirectRefundChannelBalance,
		wechatcontracts.DirectRefundChannelOtherBalance,
		wechatcontracts.DirectRefundChannelOtherBankCard,
	)
	refundStatusValues := setOf(
		wechatcontracts.DirectRefundStatusSuccess,
		wechatcontracts.DirectRefundStatusClosed,
		wechatcontracts.DirectRefundStatusProcessing,
		wechatcontracts.DirectRefundStatusAbnormal,
	)
	refundRequestFundsAccountValues := setOf(
		wechatcontracts.DirectRefundRequestFundsAccountAvailable,
		wechatcontracts.DirectRefundRequestFundsAccountUnsettle,
	)
	refundResponseFundsAccountValues := setOf(
		wechatcontracts.DirectRefundFundsAccountUnsetttled,
		wechatcontracts.DirectRefundFundsAccountAvailable,
		wechatcontracts.DirectRefundFundsAccountUnavailable,
		wechatcontracts.DirectRefundFundsAccountOperation,
		wechatcontracts.DirectRefundFundsAccountBasic,
		wechatcontracts.DirectRefundFundsAccountECNYBasic,
	)
	abnormalRefundTypeValues := setOf(
		wechatcontracts.DirectAbnormalRefundTypeUserBankCard,
		wechatcontracts.DirectAbnormalRefundTypeMerchantBankCard,
	)

	orderCreateRequestConstraints := map[string]map[string]struct{}{
		"time_expire":                    rfc3339Constraint,
		"amount.total":                   fenConstraint,
		"detail.cost_price":              fenConstraint,
		"detail.goods_detail.unit_price": fenConstraint,
	}
	orderCreateRequestEnums := map[string]map[string]struct{}{
		"support_fapiao":             booleanValues,
		"settle_info.profit_sharing": booleanValues,
		"amount.currency":            cnyValues,
	}
	orderQueryResponseConstraints := map[string]map[string]struct{}{
		"success_time":                                  rfc3339Constraint,
		"amount.total":                                  fenConstraint,
		"amount.payer_total":                            fenConstraint,
		"promotion_detail.amount":                       fenConstraint,
		"promotion_detail.wechatpay_contribute":         fenConstraint,
		"promotion_detail.merchant_contribute":          fenConstraint,
		"promotion_detail.other_contribute":             fenConstraint,
		"promotion_detail.goods_detail.unit_price":      fenConstraint,
		"promotion_detail.goods_detail.discount_amount": fenConstraint,
	}
	orderQueryResponseEnums := map[string]map[string]struct{}{
		"trade_type":                tradeTypeValues,
		"trade_state":               tradeStateValues,
		"amount.currency":           cnyValues,
		"amount.payer_currency":     cnyValues,
		"promotion_detail.scope":    promotionScopeValues,
		"promotion_detail.type":     promotionTypeValues,
		"promotion_detail.currency": cnyValues,
	}
	refundCreateRequestConstraints := map[string]map[string]struct{}{
		"amount.refund":              fenConstraint,
		"amount.from.amount":         fenConstraint,
		"amount.total":               fenConstraint,
		"goods_detail.unit_price":    fenConstraint,
		"goods_detail.refund_amount": fenConstraint,
	}
	refundCreateRequestEnums := map[string]map[string]struct{}{
		"funds_account":   refundRequestFundsAccountValues,
		"amount.currency": cnyValues,
		"amount.from.account": setOf(
			wechatcontracts.DirectRefundFundsAccountAvailable,
			wechatcontracts.DirectRefundFundsAccountUnavailable,
		),
	}
	refundResponseConstraints := map[string]map[string]struct{}{
		"success_time":                                rfc3339Constraint,
		"create_time":                                 rfc3339Constraint,
		"amount.total":                                fenConstraint,
		"amount.refund":                               fenConstraint,
		"amount.from.amount":                          fenConstraint,
		"amount.payer_total":                          fenConstraint,
		"amount.payer_refund":                         fenConstraint,
		"amount.settlement_refund":                    fenConstraint,
		"amount.settlement_total":                     fenConstraint,
		"amount.discount_refund":                      fenConstraint,
		"amount.refund_fee":                           fenConstraint,
		"promotion_detail.amount":                     fenConstraint,
		"promotion_detail.refund_amount":              fenConstraint,
		"promotion_detail.goods_detail.unit_price":    fenConstraint,
		"promotion_detail.goods_detail.refund_amount": fenConstraint,
	}
	refundResponseEnums := map[string]map[string]struct{}{
		"channel":       refundChannelValues,
		"status":        refundStatusValues,
		"funds_account": refundResponseFundsAccountValues,
		"amount.from.account": setOf(
			wechatcontracts.DirectRefundFundsAccountAvailable,
			wechatcontracts.DirectRefundFundsAccountUnavailable,
		),
		"amount.currency":        cnyValues,
		"promotion_detail.scope": promotionScopeValues,
		"promotion_detail.type":  promotionTypeValues,
	}
	refundNotifyRequestConstraints := map[string]map[string]struct{}{
		"success_time":        rfc3339Constraint,
		"amount.total":        fenConstraint,
		"amount.refund":       fenConstraint,
		"amount.payer_total":  fenConstraint,
		"amount.payer_refund": fenConstraint,
	}
	refundNotifyRequestEnums := map[string]map[string]struct{}{
		"refund_status": refundStatusValues,
	}

	return map[string]*directPaymentEndpointContract{
		endpointAuditKey("POST", "/v3/pay/transactions/jsapi"): {
			Method:             "POST",
			Path:               "/v3/pay/transactions/jsapi",
			RequestFields:      createOrderRequestFields,
			ResponseFields:     createOrderResponseFields,
			RequestConstraints: orderCreateRequestConstraints,
			RequestEnums:       orderCreateRequestEnums,
			ErrorCodes:         directPaymentCodeSetToMap(wechaterrorcodes.DirectPaymentCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/pay/transactions/id/{transaction_id}"): {
			Method:              "GET",
			Path:                "/v3/pay/transactions/id/{transaction_id}",
			RequestFields:       queryOrderByTransactionRequestFields,
			ResponseFields:      queryOrderResponseFields,
			ResponseConstraints: orderQueryResponseConstraints,
			ResponseEnums:       orderQueryResponseEnums,
			ErrorCodes:          directPaymentCodeSetToMap(wechaterrorcodes.DirectPaymentQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/pay/transactions/out-trade-no/{out_trade_no}"): {
			Method:              "GET",
			Path:                "/v3/pay/transactions/out-trade-no/{out_trade_no}",
			RequestFields:       queryOrderByOutTradeNoRequestFields,
			ResponseFields:      queryOrderResponseFields,
			ResponseConstraints: orderQueryResponseConstraints,
			ResponseEnums:       orderQueryResponseEnums,
			ErrorCodes:          directPaymentCodeSetToMap(wechaterrorcodes.DirectPaymentQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-pay/notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-pay/notify",
			RequestFields:      paymentNotifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: orderQueryResponseConstraints,
			RequestEnums:       orderQueryResponseEnums,
		},
		endpointAuditKey("POST", "/v3/pay/transactions/out-trade-no/{out_trade_no}/close"): {
			Method:         "POST",
			Path:           "/v3/pay/transactions/out-trade-no/{out_trade_no}/close",
			RequestFields:  closeOrderRequestFields,
			ResponseFields: map[string]struct{}{},
			ErrorCodes:     directPaymentCodeSetToMap(wechaterrorcodes.DirectPaymentCloseDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/refund/domestic/refunds"): {
			Method:              "POST",
			Path:                "/v3/refund/domestic/refunds",
			RequestFields:       createRefundRequestFields,
			ResponseFields:      refundResponseFields,
			RequestConstraints:  refundCreateRequestConstraints,
			ResponseConstraints: refundResponseConstraints,
			RequestEnums:        refundCreateRequestEnums,
			ResponseEnums:       refundResponseEnums,
			ErrorCodes:          directPaymentCodeSetToMap(wechaterrorcodes.DirectRefundCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/refund/domestic/refunds/{out_refund_no}"): {
			Method:              "GET",
			Path:                "/v3/refund/domestic/refunds/{out_refund_no}",
			RequestFields:       queryRefundRequestFields,
			ResponseFields:      refundResponseFields,
			ResponseConstraints: refundResponseConstraints,
			ResponseEnums:       refundResponseEnums,
			ErrorCodes:          directPaymentCodeSetToMap(wechaterrorcodes.DirectRefundQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/refund/domestic/refunds/{refund_id}/apply-abnormal-refund"): {
			Method:              "POST",
			Path:                "/v3/refund/domestic/refunds/{refund_id}/apply-abnormal-refund",
			RequestFields:       abnormalRefundRequestFields,
			ResponseFields:      refundResponseFields,
			ResponseConstraints: refundResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"type": abnormalRefundTypeValues,
			},
			ResponseEnums: refundResponseEnums,
			ErrorCodes:    directPaymentCodeSetToMap(wechaterrorcodes.DirectRefundAbnormalDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-pay/refund-notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-pay/refund-notify",
			RequestFields:      refundNotifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: refundNotifyRequestConstraints,
			RequestEnums:       refundNotifyRequestEnums,
		},
	}
}

func directPaymentCodeSetToMap(set wechaterrorcodes.DirectPaymentCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalDirectPaymentCode(code)] = struct{}{}
	}
	return result
}
