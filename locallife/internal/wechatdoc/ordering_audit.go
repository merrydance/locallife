package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type orderingEndpointContract struct {
	Method                  string
	Path                    string
	RequestFields           map[string]struct{}
	ResponseFields          map[string]struct{}
	RequestConstraints      map[string]map[string]struct{}
	ResponseConstraints     map[string]map[string]struct{}
	RequestEnums            map[string]map[string]struct{}
	ResponseEnums           map[string]map[string]struct{}
	ErrorCodes              map[string]struct{}
	CompatibilityErrorCodes map[string]struct{}
}

func AuditOrderingAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalOrderingCode)
	contractInventory := orderingContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "ordering"}

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
		compatibilityAudit := EndpointCompatibilityAudit{
			Method:                  doc.Method,
			Path:                    doc.Path,
			CompatibilityErrorCodes: diffSet(contract.CompatibilityErrorCodes, doc.ErrorCodes),
		}
		if endpointHasMissingCoverage(endpointAudit) {
			report.Endpoints = append(report.Endpoints, endpointAudit)
			accumulateAlignmentSummary(&report.Summary, endpointAudit)
		}
		if len(compatibilityAudit.CompatibilityErrorCodes) > 0 {
			report.CompatibilityGaps = append(report.CompatibilityGaps, compatibilityAudit)
			accumulateCompatibilitySummary(&report.Summary, compatibilityAudit)
		}
	}

	return report
}

func orderingContractInventory() map[string]*orderingEndpointContract {
	partnerCreateRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.PartnerJSAPIOrderRequestBody{}))
	partnerCreateResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.PartnerJSAPIOrderResponse{}))
	partnerQueryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.PartnerOrderQueryResponse{}))
	partnerNotifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.PartnerPaymentNotificationResource{}))
	partnerCloseRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.PartnerCloseOrderRequest{}))
	combineCreateRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.CombineOrderRequestBody{}))
	combineCreateResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.CombineOrderResponse{}))
	combineQueryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.CombineQueryResponseBody{}))
	combineNotifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.CombinePaymentNotification{}))
	combineCloseRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.CombineCloseOrderRequest{}))

	partnerQueryRequestFields := map[string]struct{}{
		"transaction_id": {},
		"sp_mchid":       {},
		"sub_mchid":      {},
	}
	partnerQueryByNoRequestFields := map[string]struct{}{
		"out_trade_no": {},
		"sp_mchid":     {},
		"sub_mchid":    {},
	}
	partnerCloseRequestFields["out_trade_no"] = struct{}{}
	combineQueryRequestFields := map[string]struct{}{"combine_out_trade_no": {}}
	combineCloseRequestFields["combine_out_trade_no"] = struct{}{}

	booleanValues := setOf("true", "false")
	cnyValues := setOf("CNY")
	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	partnerTradeStateValues := setOf("SUCCESS", "REFUND", "NOTPAY", "CLOSED", "REVOKED", "USERPAYING", "PAYERROR")
	partnerTradeTypeValues := setOf("JSAPI", "NATIVE", "APP", "MICROPAY", "MWEB", "FACEPAY")
	combineTradeStateValues := setOf("SUCCESS", "REFUND", "NOTPAY", "CLOSED", "PAYERROR")
	combineTradeTypeValues := setOf("NATIVE", "JSAPI", "APP", "MWEB")
	promotionScopeValues := setOf("GLOBAL", "SINGLE")
	promotionTypeValues := setOf("CASH", "NOCASH")
	partnerQueryConstraints := map[string]map[string]struct{}{
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
	partnerQueryEnums := map[string]map[string]struct{}{
		"trade_type":                partnerTradeTypeValues,
		"trade_state":               partnerTradeStateValues,
		"amount.currency":           cnyValues,
		"amount.payer_currency":     cnyValues,
		"promotion_detail.scope":    promotionScopeValues,
		"promotion_detail.type":     promotionTypeValues,
		"promotion_detail.currency": cnyValues,
	}
	combineQueryConstraints := map[string]map[string]struct{}{
		"sub_orders.success_time":                                  rfc3339Constraint,
		"sub_orders.amount.total_amount":                           fenConstraint,
		"sub_orders.amount.payer_amount":                           fenConstraint,
		"sub_orders.promotion_detail.amount":                       fenConstraint,
		"sub_orders.promotion_detail.wechatpay_contribute":         fenConstraint,
		"sub_orders.promotion_detail.merchant_contribute":          fenConstraint,
		"sub_orders.promotion_detail.other_contribute":             fenConstraint,
		"sub_orders.promotion_detail.goods_detail.unit_price":      fenConstraint,
		"sub_orders.promotion_detail.goods_detail.discount_amount": fenConstraint,
	}
	combineQueryEnums := map[string]map[string]struct{}{
		"sub_orders.trade_state":               combineTradeStateValues,
		"sub_orders.trade_type":                combineTradeTypeValues,
		"sub_orders.amount.currency":           cnyValues,
		"sub_orders.amount.payer_currency":     cnyValues,
		"sub_orders.promotion_detail.scope":    promotionScopeValues,
		"sub_orders.promotion_detail.type":     promotionTypeValues,
		"sub_orders.promotion_detail.currency": cnyValues,
	}

	return map[string]*orderingEndpointContract{
		endpointAuditKey("POST", "/v3/pay/partner/transactions/jsapi"): {
			Method:         "POST",
			Path:           "/v3/pay/partner/transactions/jsapi",
			RequestFields:  partnerCreateRequestFields,
			ResponseFields: partnerCreateResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"time_expire":                    rfc3339Constraint,
				"settle_info.subsidy_amount":     fenConstraint,
				"amount.total":                   fenConstraint,
				"detail.cost_price":              fenConstraint,
				"detail.goods_detail.unit_price": fenConstraint,
			},
			RequestEnums: map[string]map[string]struct{}{
				"settle_info.profit_sharing": booleanValues,
				"support_fapiao":             booleanValues,
				"amount.currency":            cnyValues,
			},
			ErrorCodes: orderingCodeSetToMap(wechaterrorcodes.PartnerSingleCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/pay/partner/transactions/id/{transaction_id}"): {
			Method:                  "GET",
			Path:                    "/v3/pay/partner/transactions/id/{transaction_id}",
			RequestFields:           partnerQueryRequestFields,
			ResponseFields:          partnerQueryResponseFields,
			ResponseConstraints:     partnerQueryConstraints,
			ResponseEnums:           partnerQueryEnums,
			ErrorCodes:              orderingCodeSetToMap(wechaterrorcodes.PartnerSingleQueryDocumentedCodes),
			CompatibilityErrorCodes: orderingCodeSetToMap(wechaterrorcodes.PartnerSingleQueryCompatibilityCodes),
		},
		endpointAuditKey("GET", "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}"): {
			Method:                  "GET",
			Path:                    "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}",
			RequestFields:           partnerQueryByNoRequestFields,
			ResponseFields:          partnerQueryResponseFields,
			ResponseConstraints:     partnerQueryConstraints,
			ResponseEnums:           partnerQueryEnums,
			ErrorCodes:              orderingCodeSetToMap(wechaterrorcodes.PartnerSingleQueryDocumentedCodes),
			CompatibilityErrorCodes: orderingCodeSetToMap(wechaterrorcodes.PartnerSingleQueryCompatibilityCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-ecommerce/payment-notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-ecommerce/payment-notify",
			RequestFields:      partnerNotifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: partnerQueryConstraints,
			RequestEnums:       partnerQueryEnums,
		},
		endpointAuditKey("POST", "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close"): {
			Method:                  "POST",
			Path:                    "/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close",
			RequestFields:           partnerCloseRequestFields,
			ResponseFields:          map[string]struct{}{},
			ErrorCodes:              orderingCodeSetToMap(wechaterrorcodes.PartnerSingleCloseDocumentedCodes),
			CompatibilityErrorCodes: orderingCodeSetToMap(wechaterrorcodes.PartnerSingleCloseCompatibilityCodes),
		},
		endpointAuditKey("POST", "/v3/combine-transactions/jsapi"): {
			Method:         "POST",
			Path:           "/v3/combine-transactions/jsapi",
			RequestFields:  combineCreateRequestFields,
			ResponseFields: combineCreateResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"time_start":                            rfc3339Constraint,
				"time_expire":                           rfc3339Constraint,
				"sub_orders.amount.total_amount":        fenConstraint,
				"sub_orders.settle_info.subsidy_amount": fenConstraint,
			},
			RequestEnums: map[string]map[string]struct{}{
				"sub_orders.amount.currency":            cnyValues,
				"sub_orders.settle_info.profit_sharing": booleanValues,
			},
			ErrorCodes: orderingCodeSetToMap(wechaterrorcodes.CombineCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}"): {
			Method:              "GET",
			Path:                "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}",
			RequestFields:       combineQueryRequestFields,
			ResponseFields:      combineQueryResponseFields,
			ResponseConstraints: combineQueryConstraints,
			ResponseEnums:       combineQueryEnums,
			ErrorCodes:          orderingCodeSetToMap(wechaterrorcodes.CombineQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-ecommerce/combine-notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-ecommerce/combine-notify",
			RequestFields:      combineNotifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: combineQueryConstraints,
			RequestEnums:       combineQueryEnums,
		},
		endpointAuditKey("POST", "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close"): {
			Method:                  "POST",
			Path:                    "/v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close",
			RequestFields:           combineCloseRequestFields,
			ResponseFields:          map[string]struct{}{},
			ErrorCodes:              orderingCodeSetToMap(wechaterrorcodes.CombineCloseDocumentedCodes),
			CompatibilityErrorCodes: orderingCodeSetToMap(wechaterrorcodes.CombineCloseCompatibilityCodes),
		},
	}
}

func orderingCodeSetToMap(set wechaterrorcodes.CodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalOrderingCode(code)] = struct{}{}
	}
	return result
}
