package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type refundEndpointContract struct {
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

func AuditRefundAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalRefundCode)
	contractInventory := refundContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "refund"}

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

func refundContractInventory() map[string]*refundEndpointContract {
	createRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundRequest{}))
	createResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundCreateResponse{}))
	queryByIDRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundQueryByIDRequest{}))
	queryByOutRefundNoRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundQueryByOutRefundNoRequest{}))
	queryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundQueryResponse{}))
	abnormalRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceAbnormalRefundRequest{}))
	notifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundNotification{}))
	advanceReturnQueryRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundAdvanceReturnQueryRequest{}))
	advanceReturnCreateRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundAdvanceReturnRequest{}))
	advanceReturnResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceRefundAdvanceReturnResponse{}))

	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	amountAccountValues := setOf(
		wechatcontracts.EcommerceRefundAmountAccountAvailable,
		wechatcontracts.EcommerceRefundAmountAccountUnavailable,
	)
	cnyValues := setOf(wechatcontracts.EcommerceRefundCurrencyCNY)
	statusValues := setOf(
		wechatcontracts.EcommerceRefundStatusSuccess,
		wechatcontracts.EcommerceRefundStatusClosed,
		wechatcontracts.EcommerceRefundStatusProcessing,
		wechatcontracts.EcommerceRefundStatusAbnormal,
	)
	channelValues := setOf(
		wechatcontracts.EcommerceRefundChannelOriginal,
		wechatcontracts.EcommerceRefundChannelBalance,
		wechatcontracts.EcommerceRefundChannelOtherBalance,
		wechatcontracts.EcommerceRefundChannelOtherBankCard,
	)
	promotionScopeValues := setOf(
		wechatcontracts.EcommerceRefundPromotionScopeGlobal,
		wechatcontracts.EcommerceRefundPromotionScopeSingle,
	)
	promotionTypeValues := setOf(
		wechatcontracts.EcommerceRefundPromotionTypeCoupon,
		wechatcontracts.EcommerceRefundPromotionTypeDiscount,
	)
	refundSourceValues := setOf(
		wechatcontracts.EcommerceRefundSourcePartnerAdvance,
		wechatcontracts.EcommerceRefundSourceSubMerchant,
	)
	requestFundsAccountValues := setOf(wechatcontracts.EcommerceRefundFundsAccountAvailable)
	responseFundsAccountValues := setOf(
		wechatcontracts.EcommerceRefundFundsAccountUnsetttled,
		wechatcontracts.EcommerceRefundFundsAccountAvailable,
		wechatcontracts.EcommerceRefundFundsAccountUnavailable,
		wechatcontracts.EcommerceRefundFundsAccountOperation,
		wechatcontracts.EcommerceRefundFundsAccountBasic,
		wechatcontracts.EcommerceRefundFundsAccountECNYBasic,
	)
	abnormalTypeValues := setOf(
		wechatcontracts.EcommerceAbnormalRefundTypeUserBankCard,
		wechatcontracts.EcommerceAbnormalRefundTypeMerchantBankCard,
	)
	advanceAccountValues := setOf(
		wechatcontracts.EcommerceRefundAdvanceAccountBasic,
		wechatcontracts.EcommerceRefundAdvanceAccountOperation,
	)
	advanceResultValues := setOf(
		wechatcontracts.EcommerceRefundAdvanceReturnResultSuccess,
		wechatcontracts.EcommerceRefundAdvanceReturnResultFailed,
		wechatcontracts.EcommerceRefundAdvanceReturnResultProcessing,
	)

	createRequestConstraints := map[string]map[string]struct{}{
		"amount.refund":      fenConstraint,
		"amount.from.amount": fenConstraint,
		"amount.total":       fenConstraint,
	}
	createResponseConstraints := map[string]map[string]struct{}{
		"create_time":                    rfc3339Constraint,
		"amount.refund":                  fenConstraint,
		"amount.from.amount":             fenConstraint,
		"amount.payer_refund":            fenConstraint,
		"amount.discount_refund":         fenConstraint,
		"amount.advance":                 fenConstraint,
		"promotion_detail.amount":        fenConstraint,
		"promotion_detail.refund_amount": fenConstraint,
	}
	queryResponseConstraints := map[string]map[string]struct{}{
		"success_time":                   rfc3339Constraint,
		"create_time":                    rfc3339Constraint,
		"amount.refund":                  fenConstraint,
		"amount.from.amount":             fenConstraint,
		"amount.payer_refund":            fenConstraint,
		"amount.discount_refund":         fenConstraint,
		"amount.advance":                 fenConstraint,
		"promotion_detail.amount":        fenConstraint,
		"promotion_detail.refund_amount": fenConstraint,
	}
	notifyRequestConstraints := map[string]map[string]struct{}{
		"success_time":        rfc3339Constraint,
		"amount.total":        fenConstraint,
		"amount.refund":       fenConstraint,
		"amount.payer_total":  fenConstraint,
		"amount.payer_refund": fenConstraint,
	}
	advanceReturnResponseConstraints := map[string]map[string]struct{}{
		"return_amount": fenConstraint,
		"success_time":  rfc3339Constraint,
	}

	return map[string]*refundEndpointContract{
		endpointAuditKey("POST", "/v3/ecommerce/refunds/apply"): {
			Method:              "POST",
			Path:                "/v3/ecommerce/refunds/apply",
			RequestFields:       createRequestFields,
			ResponseFields:      createResponseFields,
			RequestConstraints:  createRequestConstraints,
			ResponseConstraints: createResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"amount.from.account": amountAccountValues,
				"amount.currency":     cnyValues,
				"refund_account":      refundSourceValues,
				"funds_account":       requestFundsAccountValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"amount.from.account":    amountAccountValues,
				"amount.currency":        cnyValues,
				"promotion_detail.scope": promotionScopeValues,
				"promotion_detail.type":  promotionTypeValues,
				"refund_account":         refundSourceValues,
			},
			ErrorCodes: refundCodeSetToMap(wechaterrorcodes.EcommerceRefundCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/refunds/id/{refund_id}"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/refunds/id/{refund_id}",
			RequestFields:       queryByIDRequestFields,
			ResponseFields:      queryResponseFields,
			ResponseConstraints: queryResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"channel":                channelValues,
				"status":                 statusValues,
				"amount.from.account":    amountAccountValues,
				"amount.currency":        cnyValues,
				"promotion_detail.scope": promotionScopeValues,
				"promotion_detail.type":  promotionTypeValues,
				"refund_account":         refundSourceValues,
				"funds_account":          responseFundsAccountValues,
			},
			ErrorCodes: refundCodeSetToMap(wechaterrorcodes.EcommerceRefundQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/refunds/out-refund-no/{out_refund_no}"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/refunds/out-refund-no/{out_refund_no}",
			RequestFields:       queryByOutRefundNoRequestFields,
			ResponseFields:      queryResponseFields,
			ResponseConstraints: queryResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"channel":                channelValues,
				"status":                 statusValues,
				"amount.from.account":    amountAccountValues,
				"amount.currency":        cnyValues,
				"promotion_detail.scope": promotionScopeValues,
				"promotion_detail.type":  promotionTypeValues,
				"refund_account":         refundSourceValues,
				"funds_account":          responseFundsAccountValues,
			},
			ErrorCodes: refundCodeSetToMap(wechaterrorcodes.EcommerceRefundQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/refunds/{refund_id}/apply-abnormal-refund"): {
			Method:              "POST",
			Path:                "/v3/ecommerce/refunds/{refund_id}/apply-abnormal-refund",
			RequestFields:       abnormalRequestFields,
			ResponseFields:      queryResponseFields,
			ResponseConstraints: queryResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"type": abnormalTypeValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"channel":                channelValues,
				"status":                 statusValues,
				"amount.from.account":    amountAccountValues,
				"amount.currency":        cnyValues,
				"promotion_detail.scope": promotionScopeValues,
				"promotion_detail.type":  promotionTypeValues,
				"refund_account":         refundSourceValues,
				"funds_account":          responseFundsAccountValues,
			},
			ErrorCodes: refundCodeSetToMap(wechaterrorcodes.EcommerceRefundAbnormalDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/refunds/{refund_id}/return-advance"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/refunds/{refund_id}/return-advance",
			RequestFields:       advanceReturnQueryRequestFields,
			ResponseFields:      advanceReturnResponseFields,
			ResponseConstraints: advanceReturnResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"payer_account": advanceAccountValues,
				"payee_account": advanceAccountValues,
				"result":        advanceResultValues,
			},
			ErrorCodes: refundCodeSetToMap(wechaterrorcodes.EcommerceRefundAdvanceReturnQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/refunds/{refund_id}/return-advance"): {
			Method:              "POST",
			Path:                "/v3/ecommerce/refunds/{refund_id}/return-advance",
			RequestFields:       advanceReturnCreateRequestFields,
			ResponseFields:      advanceReturnResponseFields,
			ResponseConstraints: advanceReturnResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"payer_account": advanceAccountValues,
				"payee_account": advanceAccountValues,
				"result":        advanceResultValues,
			},
			ErrorCodes: refundCodeSetToMap(wechaterrorcodes.EcommerceRefundAdvanceReturnCreateDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-ecommerce/refund-notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-ecommerce/refund-notify",
			RequestFields:      notifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: notifyRequestConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"refund_status": setOf(
					wechatcontracts.EcommerceRefundStatusSuccess,
					wechatcontracts.EcommerceRefundStatusClosed,
					wechatcontracts.EcommerceRefundStatusAbnormal,
				),
				"refund_account": refundSourceValues,
			},
		},
	}
}

func refundCodeSetToMap(set wechaterrorcodes.RefundCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalRefundCode(code)] = struct{}{}
	}
	return result
}
