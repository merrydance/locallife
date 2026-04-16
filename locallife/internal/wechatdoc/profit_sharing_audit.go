package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type profitSharingEndpointContract struct {
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

func AuditProfitSharingAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalProfitSharingCode)
	contractInventory := profitSharingContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "profit_sharing"}

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

func profitSharingContractInventory() map[string]*profitSharingEndpointContract {
	createResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ProfitSharingResponse{}))
	amountsResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ProfitSharingAmountsResponse{}))
	queryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ProfitSharingQueryResponse{}))
	addReceiverResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.AddReceiverResponse{}))
	deleteReceiverResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DeleteReceiverResponse{}))
	returnResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ProfitSharingReturnResponse{}))
	notifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ProfitSharingNotification{}))

	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	receiverTypeValues := setOf(wechatcontracts.ReceiverTypeMerchant, wechatcontracts.ReceiverTypePersonal)
	relationTypeValues := setOf(
		wechatcontracts.RelationSupplier,
		wechatcontracts.RelationDistributor,
		wechatcontracts.RelationServiceProvider,
		wechatcontracts.RelationPlatform,
		wechatcontracts.RelationOthers,
	)
	statusValues := setOf(wechatcontracts.ProfitSharingStatusProcessing, wechatcontracts.ProfitSharingStatusFinished)
	receiverResultValues := setOf(
		wechatcontracts.ProfitSharingResultPending,
		wechatcontracts.ProfitSharingResultSuccess,
		wechatcontracts.ProfitSharingResultClosed,
	)
	receiverFailReasonValues := setOf(
		wechatcontracts.ProfitSharingFailReasonAccountAbnormal,
		wechatcontracts.ProfitSharingFailReasonNoRelation,
		wechatcontracts.ProfitSharingFailReasonReceiverHighRisk,
		wechatcontracts.ProfitSharingFailReasonReceiverRealNameNotVerified,
		wechatcontracts.ProfitSharingFailReasonNoAuth,
		wechatcontracts.ProfitSharingFailReasonReceiverReceiptLimit,
		wechatcontracts.ProfitSharingFailReasonPayerAccountAbnormal,
		wechatcontracts.ProfitSharingFailReasonInvalidRequest,
	)
	abnormalStatusValues := setOf(
		wechatcontracts.ProfitSharingAbnormalStatusPending,
		wechatcontracts.ProfitSharingAbnormalStatusFinished,
		wechatcontracts.ProfitSharingAbnormalStatusClosed,
	)
	abnormalClosedReasonValues := setOf(
		wechatcontracts.ProfitSharingAbnormalClosedReasonTimeout,
		wechatcontracts.ProfitSharingAbnormalClosedReasonRestrictTransfer,
	)
	returnResultValues := setOf(
		wechatcontracts.ProfitSharingReturnResultProcessing,
		wechatcontracts.ProfitSharingReturnResultSuccess,
		wechatcontracts.ProfitSharingReturnResultFailed,
	)
	returnFailReasonValues := setOf(
		wechatcontracts.ProfitSharingReturnFailReasonAccountAbnormal,
		wechatcontracts.ProfitSharingReturnFailReasonTimeoutClosed,
		wechatcontracts.ProfitSharingReturnFailReasonPayerAccountAbnormal,
		wechatcontracts.ProfitSharingReturnFailReasonInvalidRequest,
	)

	createResponseEnums := map[string]map[string]struct{}{
		"status":                                statusValues,
		"receivers.type":                        receiverTypeValues,
		"receivers.result":                      receiverResultValues,
		"receivers.fail_reason":                 receiverFailReasonValues,
		"receivers.abnormal_status":             abnormalStatusValues,
		"receivers.funds_abnormal_closed_reason": abnormalClosedReasonValues,
	}

	queryResponseEnums := map[string]map[string]struct{}{
		"status":                                statusValues,
		"receivers.type":                        receiverTypeValues,
		"receivers.result":                      receiverResultValues,
		"receivers.fail_reason":                 receiverFailReasonValues,
		"receivers.abnormal_status":             abnormalStatusValues,
		"receivers.funds_abnormal_closed_reason": abnormalClosedReasonValues,
	}

	returnResponseEnums := map[string]map[string]struct{}{
		"result":      returnResultValues,
		"fail_reason": returnFailReasonValues,
	}

	return map[string]*profitSharingEndpointContract{
		endpointAuditKey("POST", "/v3/ecommerce/profitsharing/orders"): {
			Method: "POST",
			Path:   "/v3/ecommerce/profitsharing/orders",
			RequestFields: setOf(
				"appid",
				"sub_mchid",
				"transaction_id",
				"out_order_no",
				"receivers",
				"receivers.type",
				"receivers.receiver_account",
				"receivers.amount",
				"receivers.description",
				"receivers.receiver_name",
				"finish",
			),
			ResponseFields: createResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"receivers.amount": fenConstraint,
			},
			ResponseConstraints: map[string]map[string]struct{}{
				"receivers.amount":      fenConstraint,
				"receivers.finish_time": rfc3339Constraint,
			},
			RequestEnums: map[string]map[string]struct{}{
				"receivers.type": receiverTypeValues,
				"finish":         setOf("true", "false"),
			},
			ResponseEnums: createResponseEnums,
			ErrorCodes:    profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/profitsharing/orders"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/profitsharing/orders",
			RequestFields:  setOf("sub_mchid", "transaction_id", "out_order_no"),
			ResponseFields: queryResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{
				"receivers.amount":      fenConstraint,
				"receivers.finish_time": rfc3339Constraint,
				"finish_amount":         fenConstraint,
			},
			ResponseEnums: queryResponseEnums,
			ErrorCodes:    profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/profitsharing/orders/{transaction_id}/amounts"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/profitsharing/orders/{transaction_id}/amounts",
			RequestFields:  setOf("transaction_id"),
			ResponseFields: amountsResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{
				"unsplit_amount": fenConstraint,
			},
			ErrorCodes: profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingAmountsDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/profitsharing/finish-order"): {
			Method:         "POST",
			Path:           "/v3/ecommerce/profitsharing/finish-order",
			RequestFields:  setOf("sub_mchid", "transaction_id", "out_order_no", "description"),
			ResponseFields: createResponseFields,
			ErrorCodes:     profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingFinishDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/profitsharing/receivers/add"): {
			Method:         "POST",
			Path:           "/v3/ecommerce/profitsharing/receivers/add",
			RequestFields:  setOf("appid", "type", "account", "name", "relation_type"),
			ResponseFields: addReceiverResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"type":          receiverTypeValues,
				"relation_type": relationTypeValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"type": receiverTypeValues,
			},
			ErrorCodes: profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingAddReceiverDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/profitsharing/receivers/delete"): {
			Method:         "POST",
			Path:           "/v3/ecommerce/profitsharing/receivers/delete",
			RequestFields:  setOf("appid", "type", "account"),
			ResponseFields: deleteReceiverResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"type": receiverTypeValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"type": receiverTypeValues,
			},
			ErrorCodes: profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingDeleteReceiverDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/profitsharing/returnorders"): {
			Method: "POST",
			Path:   "/v3/ecommerce/profitsharing/returnorders",
			RequestFields: setOf(
				"sub_mchid",
				"order_id",
				"out_order_no",
				"out_return_no",
				"return_mchid",
				"amount",
				"description",
				"transaction_id",
			),
			ResponseFields: returnResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"amount": fenConstraint,
			},
			ResponseConstraints: map[string]map[string]struct{}{
				"amount":      fenConstraint,
				"finish_time": rfc3339Constraint,
			},
			ResponseEnums: returnResponseEnums,
			ErrorCodes:    profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingReturnCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/profitsharing/returnorders"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/profitsharing/returnorders",
			RequestFields:  setOf("sub_mchid", "out_return_no", "order_id", "out_order_no"),
			ResponseFields: returnResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{
				"amount":      fenConstraint,
				"finish_time": rfc3339Constraint,
			},
			ResponseEnums: returnResponseEnums,
			ErrorCodes:    profitSharingCodeSetToMap(wechaterrorcodes.ProfitSharingReturnQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-ecommerce/profit-sharing-notify"): {
			Method:         "POST",
			Path:           "/v1/webhooks/wechat-ecommerce/profit-sharing-notify",
			RequestFields:  notifyRequestFields,
			ResponseFields: map[string]struct{}{},
			RequestConstraints: map[string]map[string]struct{}{
				"receiver.amount": fenConstraint,
				"success_time":    rfc3339Constraint,
			},
			RequestEnums: map[string]map[string]struct{}{
				"receiver.type": receiverTypeValues,
			},
		},
	}
}

func profitSharingCodeSetToMap(set wechaterrorcodes.ProfitSharingCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalProfitSharingCode(code)] = struct{}{}
	}
	return result
}